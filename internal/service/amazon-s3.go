package service

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

// aws-sdk-go-v2 documents that Key must start with /, but the sdk prefixes the key with a / always anyways, breaking GetObject and HeadObject
// listobjects does not seem to share the issue, and works with or without the leading /

// S3get returns a specified object from Amazon S3
func (c client) S3get(ctx context.Context, bucket, key string, rangeHeader *string) (*s3.GetObjectOutput, error) {
	req := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(strings.TrimLeft(key, "/")),
		Range:  rangeHeader,
	}
	return c.Client.GetObject(ctx, req)
}

// S3head returns a specified object metadata from Amazon S3
func (c client) S3head(ctx context.Context, bucket, key string, rangeHeader *string) (*s3.HeadObjectOutput, error) {
	req := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(strings.TrimLeft(key, "/")),
		Range:  rangeHeader,
	}
	return c.Client.HeadObject(ctx, req)
}

// S3exists returns true if a specified key exists in Amazon S3
func (c client) S3exists(ctx context.Context, bucket, key string) bool {
	req := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(strings.TrimLeft(key, "/")),
	}

	output, err := c.Client.HeadObject(ctx, req)
	if err != nil {
		return false
	}
	return output.ContentLength != nil && *output.ContentLength > 0
}

// S3listObjects returns a list of s3 objects
func (c client) S3listObjects(ctx context.Context, bucket, prefix string) (*s3.ListObjectsV2Output, error) {
	req := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(strings.TrimLeft(prefix, "/")),
		Delimiter: aws.String("/"),
	}
	// List 1000 records
	if !config.Config.AllPagesInDir {
		return c.Client.ListObjectsV2(ctx, req)
	}
	// List all objects with pagination
	result := &s3.ListObjectsV2Output{
		CommonPrefixes: []types.CommonPrefix{},
		Contents:       []types.Object{},
		Prefix:         aws.String(strings.TrimLeft(prefix, "/")),
	}

	paginator := s3.NewListObjectsV2Paginator(c.Client, req)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		result.CommonPrefixes = append(result.CommonPrefixes, page.CommonPrefixes...)
		result.Contents = append(result.Contents, page.Contents...)
	}

	return result, nil
}
