package service

import (
	"context"
	"crypto/tls"
	"math/rand"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

var cfg aws.Config
var cfgTime time.Time = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func awsSession(ctx context.Context, region *string) aws.Config {
	//var cfg aws.Config
	var err error

	secs := time.Since(cfgTime).Seconds()
	if secs < 65 { // Quick return
		return cfg
	}

	// only allow an increasing amount to skip this so we do not suddenly hammer imds endpoint and slow everything down
	// aws-sdk refreshs token 5min before expired, so we need to skip this atleast once in the 5min timeframe
	if rand.Float64() * secs < 60 && secs < 250 {
		return cfg
	}

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

	cfgTime = time.Now()
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
