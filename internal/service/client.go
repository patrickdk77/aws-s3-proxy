package service

import (
	"context"

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

// NewClient returns new AWS client
func NewClient(ctx context.Context, region *string) AWS {
	cfg := awsSession(ctx, region)
	return client{Context: ctx, Client: s3.NewFromConfig(cfg, func(o *s3.Options) {
		if len(config.Config.AwsAPIEndpoint) > 0 {
			o.BaseEndpoint = aws.String(config.Config.AwsAPIEndpoint)
			o.UsePathStyle = true
		}
	})}
}
