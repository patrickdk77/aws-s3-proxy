package service

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
)

// GuessBucketRegion returns a region of the bucket
func GuessBucketRegion(bucket string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg := awsSession(ctx, nil)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if len(config.Config.AwsAPIEndpoint) > 0 {
			o.BaseEndpoint = aws.String(config.Config.AwsAPIEndpoint)
			o.UsePathStyle = true
		}
	})
	return manager.GetBucketRegion(ctx, client, bucket)
}
