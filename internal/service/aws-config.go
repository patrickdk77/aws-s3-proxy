package service

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

var (
	cfgOnce sync.Once
	cfg     aws.Config
	cfgErr  error
)

// AWSConfig loads the AWS configuration on first use.
func AWSConfig() (aws.Config, error) {
	cfgOnce.Do(func() {
		cfg, cfgErr = awsconfig.LoadDefaultConfig(
			context.Background(),
			awsconfig.WithHTTPClient(configureClient()))
	})
	return cfg, cfgErr
}

func configureClient() *http.Client {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: 0,
	}
	if config.Config.InsecureTLS {
		tlsCfg.InsecureSkipVerify = true
	}
	c := config.Config
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = c.MaxIdleConns
	transport.MaxIdleConnsPerHost = c.MaxIdleConns
	transport.IdleConnTimeout = c.IdleConnTimeout
	transport.DisableCompression = c.DisableCompression
	transport.TLSClientConfig = tlsCfg
	transport.ForceAttemptHTTP2 = false
	return &http.Client{
		Transport: transport,
	}
}
