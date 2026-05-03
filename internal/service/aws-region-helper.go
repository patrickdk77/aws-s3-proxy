package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

const bucketRegionHeader = "X-Amz-Bucket-Region"

// GuessBucketRegion returns a region of the bucket.
// This replicates the logic of manager.GetBucketRegion from feature/s3/manager,
// using HeadBucket directly to avoid the dependency on that package as it was deprecated in the SDK.
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

	var capture deserializeBucketRegion
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, capture.RegisterMiddleware)
		o.Credentials = nil
	})

	if len(capture.BucketRegion) == 0 && err != nil {
		var httpStatusErr interface {
			HTTPStatusCode() int
		}
		if !errors.As(err, &httpStatusErr) {
			return "", err
		}
		if httpStatusErr.HTTPStatusCode() == http.StatusNotFound {
			return "", fmt.Errorf("bucket not found: %s", bucket)
		}
		return "", err
	}

	return capture.BucketRegion, nil
}

type deserializeBucketRegion struct {
	BucketRegion string
}

func (d *deserializeBucketRegion) RegisterMiddleware(stack *middleware.Stack) error {
	return stack.Deserialize.Add(d, middleware.After)
}

func (d *deserializeBucketRegion) ID() string {
	return "DeserializeBucketRegion"
}

func (d *deserializeBucketRegion) HandleDeserialize(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
	out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {
	out, metadata, err = next.HandleDeserialize(ctx, in)
	if err != nil {
		return out, metadata, err
	}

	resp, ok := out.RawResponse.(*smithyhttp.Response)
	if !ok {
		return out, metadata, fmt.Errorf("unknown transport type %T", out.RawResponse)
	}

	d.BucketRegion = resp.Header.Get(bucketRegionHeader)

	return out, metadata, err
}
