package controllers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func listing(t *testing.T, format, path string,
	keys ...string) string {
	t.Helper()
	m := setupMock(t, 0)
	config.Config.DirectoryListing = true
	config.Config.DirListingFormat = format
	out := &s3.ListObjectsV2Output{}
	for _, k := range keys {
		out.Contents = append(out.Contents, types.Object{
			Key:          aws.String(k),
			Size:         aws.Int64(1),
			LastModified: aws.Time(time.Unix(0, 0)),
		})
	}
	m.On("S3listObjects", mock.Anything, "bucket", mock.Anything).
		Return(out, nil)
	rr := httptest.NewRecorder()
	AwsS3(rr, httptest.NewRequest("GET", path, nil))
	return rr.Body.String()
}

func TestListingEscapesHrefs(t *testing.T) {
	for _, f := range []string{"html", "apache", "shtml"} {
		body := listing(t, f, "/d/", "d/a b#c?d%e.txt")
		want := `<a href="a%20b%23c%3Fd%25e.txt">` +
			`a b#c?d%e.txt</a>`
		assert.Contains(t, body, want, f)
	}
}

func TestListingPlainNamesUnchanged(t *testing.T) {
	body := listing(t, "shtml", "/d/", "d/file-1.tar.gz")
	assert.Contains(t, body,
		`<a href="file-1.tar.gz">file-1.tar.gz</a>`)
}

func TestListingDirectoryLink(t *testing.T) {
	m := setupMock(t, 0)
	config.Config.DirectoryListing = true
	config.Config.DirListingFormat = "html"
	m.On("S3listObjects", mock.Anything, "bucket", mock.Anything).
		Return(&s3.ListObjectsV2Output{
			CommonPrefixes: []types.CommonPrefix{
				{Prefix: aws.String("d/sub dir/")},
			},
		}, nil)
	rr := httptest.NewRecorder()
	AwsS3(rr, httptest.NewRequest("GET", "/d/", nil))
	assert.Contains(t, rr.Body.String(),
		`<a href="sub%20dir/">sub dir/</a>`)
}

func TestListingObjectNameXSS(t *testing.T) {
	name := "d/<img src=x onerror=alert(1)>.txt"
	for _, f := range []string{"html", "apache", "shtml"} {
		body := listing(t, f, "/d/", name)
		assert.NotContains(t, body, "<img", f)
		assert.Contains(t, body,
			"&lt;img src=x onerror=alert(1)&gt;.txt", f)
	}
}

func TestListingPathXSS(t *testing.T) {
	body := listing(t, "apache",
		"/%3Cscript%3Ealert(2)%3C%2Fscript%3E/")
	assert.NotContains(t, body, "<script>")
	assert.Contains(t, body,
		"Index of &lt;script&gt;alert(2)&lt;/script&gt;/")
}

func TestListingAttributeBreakout(t *testing.T) {
	body := listing(t, "html", "/d/",
		`d/x" onmouseover="alert(3)`)
	assert.NotContains(t, body, `" onmouseover="`)
}

func TestListingSchemeName(t *testing.T) {
	for _, f := range []string{"html", "apache", "shtml"} {
		body := listing(t, f, "/d/", "d/javascript:alert(4)")
		assert.NotContains(t, body, `href="javascript:`, f)
		assert.Contains(t, body,
			`href="./javascript:alert%284%29"`, f)
	}
}

func TestListingJSONNames(t *testing.T) {
	body := listing(t, "", "/d/", "d/a.txt", "d/<b>.txt")
	var names []string
	assert.NoError(t, json.Unmarshal([]byte(body), &names))
	assert.Equal(t, []string{"<b>.txt", "a.txt"}, names)
}

func TestListingJSONEmpty(t *testing.T) {
	body := listing(t, "", "/d/")
	assert.JSONEq(t, `[]`, body)
}
