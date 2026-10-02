package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/patrickdk77/aws-s3-proxy/internal/metrics"

	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
)

var newClient = service.NewClient

// HealthcheckHandler wraps the content of each service dependency
type healthcheck struct {
	Healthy   bool          `json:"healthy"`
	Time      time.Duration `json:"time_ns"`
	TimeHuman int64         `json:"time_human"`
	Error     string        `json:"error,omitempty"`
}

// HealthcheckResponse struct builds the healthcheck endpoint response
type HealthcheckResponse struct {
	S3Bucket healthcheck `json:"s3_bucket"`
}

func executeHealthCheck(ctx context.Context, awsClient service.AWS) error {
	c := config.Config
	obj, err := awsClient.S3get(ctx, c.S3Bucket,
		c.S3KeyPrefix+c.HealthCheckPath, nil)

	metrics.UpdateS3Reads(err, metrics.GetObjectAction, metrics.HealthcheckSource)
	// if file exists, return ok
	if err == nil {
		_ = obj.Body.Close()
		return nil
	}
	// we have some kind of error. Normally we accept the 404 key not found because it means that we are able
	// to reach the endpoint without any issue.
	var ae smithy.APIError
	if errors.As(err, &ae) {
		if ae.ErrorCode() == "NoSuchKey" {
			return nil
		}
	}
	// Also check typed error
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return nil
	}

	return err
}

// HealthcheckHandler validates the s3 proxy dependencies and return a 500 if it is not ready to serve traffic
func HealthcheckHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	start := time.Now()
	httpRes := &HealthcheckResponse{}
	client := newClient(req.Context(),
		aws.String(config.Config.AwsRegion))
	err := executeHealthCheck(req.Context(), client)
	httpRes.S3Bucket.Time = time.Since(start)
	httpRes.S3Bucket.TimeHuman = httpRes.S3Bucket.Time.Milliseconds()

	if err == nil {
		httpRes.S3Bucket.Healthy = true
	} else {
		log.Printf("[healthcheck] %v", err)
		httpRes.S3Bucket.Error = "S3 request failed"
		var ae smithy.APIError
		if errors.As(err, &ae) {
			httpRes.S3Bucket.Error = ae.ErrorCode()
		}
	}
	// marshal response
	body, err := json.Marshal(httpRes)
	if err != nil {
		body = []byte(`{"error":"cannot marshal response"}`)
	}

	// if there was an error on unmarshaling or the end point is not healthy, then return an appropriate status code.
	statusCode := http.StatusOK
	if err != nil || !httpRes.S3Bucket.Healthy {
		statusCode = http.StatusInternalServerError
	}
	w.WriteHeader(statusCode)
	metrics.HealthCheck.WithLabelValues(strconv.Itoa(statusCode)).Inc()

	// write final result
	_, _ = w.Write(body)
}
