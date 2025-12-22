package service

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

func awsSession(ctx context.Context, region *string) aws.Config {
	var cfg aws.Config
	var err error

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithHTTPClient(configureClient()),
	}

	if region != nil {
		opts = append(opts, awsconfig.WithRegion(*region))
	}
	//opts = append(opts, awsconfig.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody))

	cfg, err = awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		// handle error panic or log fatal
		panic(err)
	}

	return cfg
}

func configureClient() *http.Client {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: 0,
	}
	if config.Config.InsecureTLS {
		tlsCfg.InsecureSkipVerify = true
	}
	transport := &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		MaxIdleConns:       config.Config.MaxIdleConns,
		IdleConnTimeout:    config.Config.IdleConnTimeout,
		DisableCompression: config.Config.DisableCompression,
		TLSClientConfig:    tlsCfg,
	}
	return &http.Client{
		Transport: transport,
	}
}
