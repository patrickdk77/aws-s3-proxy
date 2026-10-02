package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func offline(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"AWS_REGION":                  "",
		"AWS_DEFAULT_REGION":          "",
		"AWS_PROFILE":                 "",
		"AWS_CONFIG_FILE":             "/dev/null",
		"AWS_SHARED_CREDENTIALS_FILE": "/dev/null",
		"AWS_EC2_METADATA_DISABLED":   "true",
		"AWS_ACCESS_KEY_ID":           "AKIDTEST",
		"AWS_SECRET_ACCESS_KEY":       "test",
		"HTTPS_PROXY":                 "http://127.0.0.1:1",
		"HTTP_PROXY":                  "http://127.0.0.1:1",
		"NO_PROXY":                    "",
	} {
		t.Setenv(k, v)
	}
	old := *config.Config
	t.Cleanup(func() {
		*config.Config = old
		resetAWS()
	})
	resetAWS()
}

func fakeS3(t *testing.T, status int, region string) {
	t.Helper()
	h := func(w http.ResponseWriter, r *http.Request) {
		if region != "" {
			w.Header().Set("X-Amz-Bucket-Region", region)
		}
		w.WriteHeader(status)
	}
	srv := httptest.NewServer(http.HandlerFunc(h))
	t.Cleanup(srv.Close)
	config.Config.AwsAPIEndpoint = srv.URL
}

func region(c AWS) string {
	return c.(client).Options().Region
}

func TestNewClientRegion(t *testing.T) {
	offline(t)
	c := NewClient(context.Background(), aws.String("eu-west-1"))
	assert.Equal(t, "eu-west-1", region(c))
}

func TestNewClientRegionAfterGuess(t *testing.T) {
	offline(t)
	fakeS3(t, http.StatusMovedPermanently, "eu-central-1")
	_, _ = GuessBucketRegion("bucket")
	c := NewClient(context.Background(), aws.String("eu-west-1"))
	assert.Equal(t, "eu-west-1", region(c))
}

func TestNewClientReusedPerRegion(t *testing.T) {
	offline(t)
	ctx := context.Background()
	a := NewClient(ctx, aws.String("eu-west-1")).(client)
	b := NewClient(ctx, aws.String("eu-west-1")).(client)
	c := NewClient(ctx, aws.String("us-west-2")).(client)
	assert.Same(t, a.Client, b.Client)
	assert.NotSame(t, a.Client, c.Client)
	assert.Equal(t, "us-west-2", region(c))
}

func TestNewClientEndpoint(t *testing.T) {
	offline(t)
	config.Config.AwsAPIEndpoint = "http://127.0.0.1:9000"
	o := NewClient(context.Background(),
		aws.String("us-east-1")).(client).Options()
	assert.Equal(t, "http://127.0.0.1:9000",
		aws.ToString(o.BaseEndpoint))
	assert.True(t, o.UsePathStyle)
}

func TestNewClientConcurrent(t *testing.T) {
	offline(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := NewClient(context.Background(),
				aws.String("eu-west-1"))
			assert.Equal(t, "eu-west-1", region(c))
		}()
	}
	wg.Wait()
}

func TestTransportSettings(t *testing.T) {
	offline(t)
	config.Config.MaxIdleConns = 150
	tr := configureClient().Transport.(*http.Transport)
	assert.Equal(t, 150, tr.MaxIdleConns)
	assert.Equal(t, 150, tr.MaxIdleConnsPerHost)
	assert.NotZero(t, tr.TLSHandshakeTimeout)
	assert.NotNil(t, tr.DialContext)
	assert.NotNil(t, tr.Proxy)
}

func TestGuessBucketRegionNoRegionSet(t *testing.T) {
	offline(t)
	fakeS3(t, http.StatusMovedPermanently, "eu-central-1")
	got, err := GuessBucketRegion("bucket")
	assert.NoError(t, err)
	assert.Equal(t, "eu-central-1", got)
}

func TestGuessBucketRegionForbidden(t *testing.T) {
	offline(t)
	fakeS3(t, http.StatusForbidden, "ap-south-1")
	got, err := GuessBucketRegion("bucket")
	assert.NoError(t, err)
	assert.Equal(t, "ap-south-1", got)
}

func TestGuessBucketRegionNotFound(t *testing.T) {
	offline(t)
	fakeS3(t, http.StatusNotFound, "")
	got, err := GuessBucketRegion("bucket")
	assert.ErrorContains(t, err, "bucket not found")
	assert.Empty(t, got)
}

func TestGuessBucketRegionUnreachable(t *testing.T) {
	offline(t)
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	config.Config.AwsAPIEndpoint = srv.URL
	got, err := GuessBucketRegion("bucket")
	assert.Error(t, err)
	assert.Empty(t, got)
}

func TestGuessBucketRegionNoHeader(t *testing.T) {
	offline(t)
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	fakeS3(t, http.StatusInternalServerError, "")
	got, err := GuessBucketRegion("bucket")
	assert.Error(t, err)
	assert.Empty(t, got)
}
