package service

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

// AWS is a service to interact with original AWS services
type AWS interface {
	S3get(ctx context.Context, bucket, key string, rangeHeader *string) (*s3.GetObjectOutput, error)
	S3head(ctx context.Context, bucket, key string, rangeHeader *string) (*s3.HeadObjectOutput, error)
	S3exists(ctx context.Context, bucket, key string) bool
	S3listObjects(ctx context.Context, bucket, prefix string) (*s3.ListObjectsV2Output, error)
}

type client struct {
	context.Context
	*s3.Client
}

var clients sync.Map

// NewClient returns new AWS client
func NewClient(ctx context.Context, region *string) AWS {
	r := aws.ToString(region)
	if c, ok := clients.Load(r); ok {
		return client{Context: ctx, Client: c.(*s3.Client)}
	}
	cfg, err := AWSConfig()
	if err != nil {
		panic(err)
	}
	opts := func(o *s3.Options) {
		if r != "" {
			o.Region = r
		}
		setEndpoint(o)
		o.DisableLogOutputChecksumValidationSkipped = true
	}
	c, _ := clients.LoadOrStore(r, s3.NewFromConfig(cfg, opts))
	return client{Context: ctx, Client: c.(*s3.Client)}
}

func setEndpoint(o *s3.Options) {
	if ep := config.Config.AwsAPIEndpoint; ep != "" {
		o.BaseEndpoint = aws.String(ep)
		o.UsePathStyle = true
	}
}
