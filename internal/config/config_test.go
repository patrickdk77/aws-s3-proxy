package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func defaultConfig() *config {
	return &config{
		AwsRegion:          "",
		AwsAPIEndpoint:     "",
		S3Bucket:           "",
		S3KeyPrefix:        "",
		IndexDocument:      "index.html",
		DirectoryListing:   false,
		DirListingFormat:   "",
		HTTPCacheControl:   "",
		HTTPExpires:        "",
		BasicAuthUser:      "",
		BasicAuthPass:      "",
		Port:               "80",
		Host:               "",
		AccessLog:          false,
		SslCert:            "",
		SslKey:             "",
		StripPath:          "",
		ContentEncoding:    true,
		CorsAllowOrigin:    "",
		CorsAllowMethods:   "",
		CorsAllowHeaders:   "",
		CorsMaxAge:         int64(600),
		HealthCheckPath:    "",
		AllPagesInDir:      false,
		MaxIdleConns:       150,
		IdleConnTimeout:    time.Duration(10) * time.Second,
		DisableCompression: true,
		InsecureTLS:        false,
	}
}

func TestConfigDefaults(t *testing.T) {
	expected := defaultConfig()
	assert.Equal(t, expected, Config)
}

func TestChangeDefaults(t *testing.T) {
	_ = os.Setenv("DIRECTORY_LISTINGS", "1")
	_ = os.Setenv("ACCESS_LOG", "True")
	_ = os.Setenv("CONTENT_ENCODING", "f")
	_ = os.Setenv("CORS_MAX_AGE", "0")
	_ = os.Setenv("GET_ALL_PAGES_IN_DIR", "TRUE")
	_ = os.Setenv("MAX_IDLE_CONNECTIONS", "0")
	_ = os.Setenv("IDLE_CONNECTION_TIMEOUT", "60")
	_ = os.Setenv("DISABLE_COMPRESSION", "FALSE")
	_ = os.Setenv("INSECURE_TLS", "t")

	Setup()

	expected := defaultConfig()
	expected.DirectoryListing = true
	expected.AccessLog = true
	expected.ContentEncoding = false
	expected.CorsMaxAge = 0
	expected.AllPagesInDir = true
	expected.MaxIdleConns = 0
	expected.IdleConnTimeout = time.Duration(60) * time.Second
	expected.DisableCompression = false
	expected.InsecureTLS = true

	assert.Equal(t, expected, Config)
}
