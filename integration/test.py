import contextlib
import datetime
import functools
import hashlib
import http.server
import json
import mimetypes
import shlex
import socket
import socketserver
import subprocess
import threading
import time
import uuid
from typing import Any

import boto3
import httpx2
import pytest
from jwcrypto import jwk, jwt
from moto import mock_aws
from moto.server import ThreadedMotoServer

S3_PROXY_PORT = 9000
JWKS_SERVER_PORT = 9001
JWKS_PATH = "/jwks"
JWT_AUDIENCE = "test-aud"
JWT_HEADER = "jwt-assertion"
JWT_KEY_ID = "test-key-id"
S3_BUCKET_NAME = "test-bucket"


@pytest.fixture(scope="session")
def image_name(pytestconfig):
    return pytestconfig.getoption("image")


@pytest.fixture(scope="session", autouse=True)
def moto_server_port():
    server = ThreadedMotoServer(port=0)
    server.start()
    host, port = server.get_host_and_port()
    endpoint_url = f"http://{host}:{port}"
    print("Mock AWS server listening at", endpoint_url)

    yield port

    server.stop()


@pytest.fixture
def s3_bucket(moto_server_port):
    @contextlib.contextmanager
    def with_s3_bucket(bucket_name, files: dict[str, str | dict[str, Any]]):
        client = boto3.client("s3", endpoint_url=f"http://localhost:{moto_server_port}")
        client.create_bucket(Bucket=bucket_name)
        for key, content in files.items():
            if isinstance(content, str):
                client.put_object(
                    Bucket=bucket_name,
                    Key=key,
                    Body=content,
                    ContentType=mimetypes.guess_type(key)[0],
                )
            else:
                client.put_object(Bucket=bucket_name, Key=key, **content)

        yield

        for key, content in files.items():
            client.delete_object(Bucket=bucket_name, Key=key)
        client.delete_bucket(Bucket=bucket_name)

    return with_s3_bucket


class JWKSHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        self.jwks_data = kwargs.pop("jwks_data")
        super().__init__(*args, **kwargs)

    def do_GET(self):
        if self.path == JWKS_PATH:
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(self.jwks_data).encode("utf-8"))
        else:
            self.send_response(404)
            self.end_headers()


class AuthServer:
    def __init__(self, port: int, key_id: str):
        self.port = port
        self.auth_keypair = jwk.JWK.generate(
            kty="RSA", size=2048, kid=key_id, use="sig", alg="RS256"
        )
        self.jwks_data = {
            "keys": [json.loads(self.auth_keypair.export(private_key=False))]
        }
        self.jwks_server = None

    def start_jwks_server(self):
        handler = functools.partial(JWKSHandler, jwks_data=self.jwks_data)
        socketserver.TCPServer.allow_reuse_address = True
        self.jwks_server = socketserver.TCPServer(("", self.port), handler)
        print("Mock JWKS server listening at port", self.port)
        self.jwks_server.serve_forever()

    def stop_jwks_server(self):
        if self.jwks_server:
            self.jwks_server.shutdown()
            self.jwks_server.server_close()

    def proxy_request(
        self,
        path: str,
        *,
        jwt_audience: str = JWT_AUDIENCE,
        jwt_issuer: str | None = None,
        method: str = "get",
        headers: dict[str, str] | None = None,
        jwt_header: bool | str = True,
    ):
        if jwt_issuer is None:
            jwt_issuer = f"http://host.docker.internal:{self.port}"
        access_token = None
        if jwt_header is True:
            access_token = self._sign_access_token(
                issuer=jwt_issuer,
                audience=jwt_audience,
                email="hello@example.com",
            )
        elif isinstance(jwt_header, str):
            access_token = jwt_header
        headers = dict(headers or {})
        if access_token is not None:
            headers[JWT_HEADER] = access_token
        return getattr(httpx2, method)(
            f"http://localhost:{S3_PROXY_PORT}{path}", headers=headers
        )

    def _sign_access_token(self, *, issuer, audience, email):
        now = int(time.time())
        claims = {
            "aud": audience,
            "email": email,
            "exp": now + 300,
            "iat": now,
            "nbf": now,
            "iss": issuer,
            "type": "app",
            "identity_nonce": "testnonce",
            "sub": "user-id",
            "country": "US",
        }
        token = jwt.JWT(
            header={"alg": self.auth_keypair["alg"], "kid": self.auth_keypair["kid"]},
            claims=claims,
        )
        token.make_signed_token(self.auth_keypair)
        return token.serialize()


@pytest.fixture(scope="session", autouse=True)
def auth_server():
    server = AuthServer(port=JWKS_SERVER_PORT, key_id=JWT_KEY_ID)
    jwks_thread = threading.Thread(target=server.start_jwks_server, daemon=True)
    jwks_thread.start()
    yield server
    server.stop_jwks_server()


@pytest.fixture(scope="session", autouse=True)
def s3_proxy_server(image_name, moto_server_port):
    container_name = f"s3-proxy-{uuid.uuid4().hex}"
    command = f"""
    docker \
    run \
    --rm \
    --add-host=host.docker.internal:host-gateway \
    --name {container_name} \
    -e AWS_ACCESS_KEY_ID=fake \
    -e AWS_SECRET_ACCESS_KEY=fake \
    -e AWS_REGION=us-west-2 \
    -e AWS_S3_BUCKET={S3_BUCKET_NAME} \
    -e AWS_API_ENDPOINT=http://host.docker.internal:{moto_server_port} \
    -e JWKS_URI=http://host.docker.internal:{JWKS_SERVER_PORT}{JWKS_PATH} \
    -e JWT_ISSUER=http://host.docker.internal:{JWKS_SERVER_PORT} \
    -e JWT_AUDIENCE={JWT_AUDIENCE} \
    -e JWT_HEADER={JWT_HEADER} \
    -e APP_PORT={S3_PROXY_PORT} \
    -e DEBUG=1 \
    -p {S3_PROXY_PORT}:{S3_PROXY_PORT} \
    {image_name}
    """
    print(command.strip())
    subprocess.Popen(shlex.split(command))

    try:
        _wait_for_local_port(S3_PROXY_PORT)
    except Exception:
        _stop_container(container_name)
        raise

    yield

    _stop_container(container_name)


class LocalPortTimeout(Exception):
    pass


def _wait_for_local_port(port: int, timeout: int = 10):
    with contextlib.closing(socket.socket(socket.AF_INET, socket.SOCK_STREAM)) as sock:
        tries = 0
        while sock.connect_ex(("localhost", port)) != 0:
            time.sleep(1)
            tries += 1
            if tries > timeout:
                raise LocalPortTimeout(
                    f"Local server did not start listening at {port} in time"
                )


def _stop_container(container_name):
    stop_response = subprocess.run(
        ["docker", "stop", container_name], capture_output=True, text=True, check=False
    )
    if "Error response" in stop_response.stderr:
        print("Failed to stop container. Stop it manually with:")
        print(f"docker stop {container_name}")


def _format_http_date(dt: datetime.datetime) -> str:
    return dt.strftime("%a, %d %b %Y %H:%M:%S GMT")


def _parse_http_date(dt: str) -> datetime.datetime:
    return datetime.datetime.strptime(dt, "%a, %d %b %Y %H:%M:%S GMT").replace(
        tzinfo=datetime.UTC
    )


def _get_etag(s: bytes) -> str:
    md5 = hashlib.md5()
    md5.update(s)
    return f'"{md5.hexdigest()}"'


@mock_aws
def test_proxy_requests(subtests, s3_bucket, auth_server):
    time_before_upload = datetime.datetime.now(datetime.UTC) - datetime.timedelta(
        days=1
    )
    with s3_bucket(
        bucket_name=S3_BUCKET_NAME,
        files={
            "index.html": "root_index_document",
            "dir/index.html": "directory_index_document",
            "foo/bar.txt": "text_document",
        },
    ):
        time_after_upload = datetime.datetime.now(datetime.UTC)

        with subtests.test("HEAD /"):
            res = auth_server.proxy_request("/", method="head")
            assert res.status_code == 200
            assert res.headers["content-type"] == "text/html"

        with subtests.test("GET / resolves to index.html"):
            res = auth_server.proxy_request("/")
            assert res.status_code == 200
            assert res.text == "root_index_document"

        with subtests.test("GET /index.html"):
            res = auth_server.proxy_request("/index.html")
            assert res.status_code == 200
            assert res.text == "root_index_document"

        with subtests.test("GET /index.html with range"):
            res = auth_server.proxy_request(
                "/index.html", headers={"range": f"bytes=0-{len('root') - 1}"}
            )
            assert res.status_code == 206
            assert res.text == "root"

        with subtests.test("GET /dir/ resolves to index.html"):
            res = auth_server.proxy_request("/dir/")
            assert res.status_code == 200
            assert res.text == "directory_index_document"

        with subtests.test("GET /dir2/ returns 404"):
            res = auth_server.proxy_request("/dir2")
            assert res.status_code == 404

        with subtests.test("GET /bar.txt returns 404"):
            res = auth_server.proxy_request("/bar.txt")
            assert res.status_code == 404
            assert res.headers["content-type"].startswith("text/plain")

        with subtests.test("GET /foo/bar.txt"):
            res = auth_server.proxy_request("/foo/bar.txt")
            assert res.status_code == 200
            assert res.text == "text_document"
            etag = res.headers["etag"]

        with subtests.test("GET /foo/bar.txt with If-Match=matching_etag"):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-Match": etag,
                },
            )
            assert res.status_code == 200
            assert res.text == "text_document"

        with subtests.test("GET /foo/bar.txt with If-Match=wrong_etag"):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-Match": etag + "xxx",
                },
            )
            assert res.status_code == 412

        with subtests.test(
            "GET /foo/bar.txt with If-Modified-Since=time_before_upload"
        ):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-Modified-Since": _format_http_date(time_before_upload),
                },
            )
            assert res.status_code == 200
            assert res.text == "text_document"

        with subtests.test("GET /foo/bar.txt with If-Modified-Since=time_after_upload"):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-Modified-Since": _format_http_date(time_after_upload),
                },
            )
            assert res.status_code == 304

        with subtests.test("GET /foo/bar.txt with If-None-Match=matching_etag"):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-None-Match": etag,
                },
            )
            assert res.status_code == 304

        with subtests.test("GET /foo/bar.txt with If-None-Match=wrong_etag"):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-None-Match": etag + "xxx",
                },
            )
            assert res.status_code == 200
            assert res.text == "text_document"

        with subtests.test(
            "GET /foo/bar.txt with If-Unmodified-Since=time_before_upload"
        ):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-Unmodified-Since": _format_http_date(time_before_upload),
                },
            )
            assert res.status_code == 412

        with subtests.test(
            "GET /foo/bar.txt with If-Unmodified-Since=time_after_upload"
        ):
            res = auth_server.proxy_request(
                "/foo/bar.txt",
                headers={
                    "If-Unmodified-Since": _format_http_date(time_after_upload),
                },
            )
            assert res.status_code == 200
            assert res.text == "text_document"

        with subtests.test("GET / with no JWT token"):
            res = auth_server.proxy_request("/foo/bar.txt", jwt_header=False)
            assert res.status_code == 401

        with subtests.test("GET / with invalid JWT token"):
            res = auth_server.proxy_request("/foo/bar.txt", jwt_header="bad_token")
            assert res.status_code == 401

        with subtests.test("GET / with wrong audience"):
            res = auth_server.proxy_request(
                "/foo/bar.txt", jwt_audience=JWT_AUDIENCE + "_bad"
            )
            assert res.status_code == 401

        with subtests.test("GET / with wrong issuer"):
            res = auth_server.proxy_request(
                "/foo/bar.txt", jwt_issuer="different_issuer"
            )
            assert res.status_code == 401


@mock_aws
def test_content_headers(subtests, s3_bucket, auth_server):
    with s3_bucket(
        bucket_name=S3_BUCKET_NAME,
        files={
            "index.html": {
                "CacheControl": "custom-control",
                "ContentDisposition": "custom-disposition",
                "ContentLanguage": "custom-language",
                "ContentLength": 5,
                "ContentType": "custom-type",
                "Expires": datetime.datetime(2001, 2, 3, 4, 5, 6),
                "Body": "hello",
            },
            "some.bin": {
                "ContentEncoding": "custom-encoding",
                "ContentLength": 5,
                "Body": "hello",
            },
        },
    ):
        now = datetime.datetime.now(datetime.UTC)

        with subtests.test("kitchensink"):
            res = auth_server.proxy_request("/")

            assert res.status_code == 200
            assert res.headers["Cache-Control"] == "custom-control"
            assert res.headers["Content-Disposition"] == "custom-disposition"
            assert res.headers["Content-Language"] == "custom-language"
            assert res.headers["Content-Length"] == "5"
            assert res.headers["Content-Type"] == "custom-type"
            assert res.headers["Expires"] == "Sat, 03 Feb 2001 04:05:06 GMT"
            assert res.text == "hello"

            assert _parse_http_date(res.headers["Last-Modified"]) == pytest.approx(
                now, abs=datetime.timedelta(seconds=10)
            )
            assert res.headers["ETag"] == _get_etag(b"hello")

        with subtests.test("some.bin"):
            res = auth_server.proxy_request("/some.bin")

            assert res.status_code == 200
            assert res.headers["Content-Encoding"] == "custom-encoding"
            assert res.headers["Content-Length"] == "5"
            assert res.text == "hello"
