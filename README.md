# Reverse proxy for AWS S3 w/ basic authentication

Added multiarch support to builds.

## Description

This is a reverse proxy for AWS S3, which is able to provide basic authentication as well.  
You don't need to configure a Bucket for `Website Hosting`.

http://this-proxy.com/access/ -> s3://bucket/access/index.html

## Usage

### 1. Set environment variables

Environment Variables     | Description                                       | Required | Default
------------------------- | ------------------------------------------------- | -------- | -----------------
AWS_S3_BUCKET             | The `S3 bucket` to be proxied with this app.      | *        |
AWS_S3_KEY_PREFIX         | You can configure `S3 object key` prefix.         |          | -
AWS_REGION                | The AWS `region` where the S3 bucket exists.      |          | us-east-1
AWS_ACCESS_KEY_ID         | AWS `access key` for API access.                  |          | EC2 Instance Role
AWS_SECRET_ACCESS_KEY     | AWS `secret key` for API access.                  |          | EC2 Instance Role
AWS_API_ENDPOINT          | The endpoint for AWS API for local development.   |          | -
INDEX_DOCUMENT            | Name of your index document.                      |          | index.html
DIRECTORY_LISTINGS        | List files when a specified URL ends with /.      |          | false
DIRECTORY_LISTINGS_FORMAT | Configures directory listing to be `html` (spider parsable) or `shtml` (pip compatible) or `apache` for apache style |          | -
DIRECTORY_LISTINGS_CHECK_INDEX | Check for `INDEX_DOCUMENT` in the folder before listing files |       | false
HTTP_CACHE_CONTROL        | Overrides S3's HTTP `Cache-Control` header.       |          | S3 Object metadata
HTTP_EXPIRES              | Overrides S3's HTTP `Expires` header.             |          | S3 Object metadata
BASIC_AUTH_USER           | User for basic authentication. Space seperated list |          | -
BASIC_AUTH_PASS           | Password for basic authentication. Space seperated list |          | -
SSL_CERT_PATH             | TLS: cert.pem file path.                          |          | -
SSL_KEY_PATH              | TLS: key.pem file path.                           |          | -
CORS_ALLOW_ORIGIN         | CORS: a URI that may access the resource.         |          | -
CORS_ALLOW_METHODS        | CORS: Comma-delimited list of the allowed [HTTP request methods](https://www.w3.org/Protocols/rfc2616/rfc2616-sec9.html). |          | -
CORS_ALLOW_HEADERS        | CORS: Comma-delimited list of the supported request headers. |          | -
CORS_MAX_AGE              | CORS: Maximum number of seconds the results of a preflight request can be cached. |          | 600
APP_PORT                  | The port number to be assigned for listening.     |          | 80
APP_HOST                  | The host name used to the listener                |          | Listens on all available unicast and anycast IP addresses of the local system.
ACCESS_LOG                | Send access logs to /dev/stdout.                  |          | false
FORWARDED_FOR             | Header name to use to parse proxied ip address from |          | -
STRIP_PATH                | Strip path prefix.                                |          | -
CONTENT_ENCODING          | Compress response data if the request allows.     |          | true
HEALTHCHECK_PATH          | If it's specified, the path always returns 200 OK  /healthz |          | -
HEALTHCHECKER_PATH        | Used by docker healthcheck script, if different from HEALTHCHECK_PATH |          | -
METRICS_PATH              | prometheus statistics /metrics                    |          | -
VERSION_PATH              | version info of proxy /version                    |          | -
GET_ALL_PAGES_IN_DIR      | If true will make several calls to get all pages of destination directory | | false
MAX_IDLE_CONNECTIONS      | Allowed number of idle connections to the S3 storage |       | 150
IDLE_CONNECTION_TIMEOUT   | Allowed timeout to the S3 storage.                |          | 10
DISABLE_COMPRESSION       | If true will pass encoded content through as-is.  |          | true
INSECURE_TLS              | If true it will skip cert checks                  |          | false
JWT_SECRET_KEY            | JSON Web Token secret key to athenticate requests |          | -
JWT_USER_FIELD            | JSON Web Token field to put in username for logs  |          | -
JWT_HEADER                | JSON Web Token header to use, instead of Authorization, aka Cf-Access-Jwt-Assertion |          | -
SPA                       | Signle Page Application - If true server will return index document content on 404 error (like `try_files $uri $uri/ /index.html;` in nginx) |          | false
WHITELIST_IP_RANGES       | commma separated list of IPs and IP ranges.       |          | -
CONTENT_TYPE              | Override the default Content-Type response header |          | -
CONTENT_DISPOSITION       | Override the default Content-Disposition response header |          | -
USERNAME_HEADER           | Username Header name, for cloudflare Cf-Access-Authenticated-User-Email |          | -

### 2. Run the application

`docker run -d -p 8080:80 -e AWS_REGION -e AWS_S3_BUCKET patrickdk/s3-proxy`

* with basic auth:

`docker run -d -p 8080:80 -e AWS_REGION -e AWS_S3_BUCKET -e BASIC_AUTH_USER -e BASIC_AUTH_PASS patrickdk/s3-proxy`

* with TLS:

`docker run -d -p 8080:80 -e AWS_REGION -e AWS_S3_BUCKET -e SSL_CERT_PATH -e SSL_KEY_PATH patrickdk/s3-proxy`

* with CORS:

`docker run -d -p 8080:80 -e CORS_ALLOW_ORIGIN -e CORS_ALLOW_METHODS -e CORS_ALLOW_HEADERS -e CORS_MAX_AGE patrickdk/s3-proxy`

* with docker-compose.yml:

```
proxy:
  image: patrickdk/s3-proxy
  ports:
    - 8080:80
  environment:
    - AWS_REGION=ap-northeast-1
    - AWS_ACCESS_KEY_ID
    - AWS_SECRET_ACCESS_KEY
    - AWS_S3_BUCKET
    - BASIC_AUTH_USER=admin
    - BASIC_AUTH_PASS=password
    - ACCESS_LOG=true
  container_name: proxy
```


## Copyright and license

Code released under the [MIT license](https://github.com/pottava/aws-s3-proxy/blob/master/LICENSE).
