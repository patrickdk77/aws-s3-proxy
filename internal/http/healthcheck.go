package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pottava/aws-s3-proxy/internal/config"
	"github.com/pottava/aws-s3-proxy/internal/service"
)

// HealthcheckResponse struct builds the healthcheck endpoint response
type HealthcheckResponse struct {
	S3Bucket healthcheck `json:"s3_bucket"`
}

// HealthcheckHandler wraps the content of each service dependency
type healthcheck struct {
	Healthy   bool          `json:"healthy"`
	Time      time.Duration `json:"time_ns"`
	TimeHuman int64         `json:"time_human"`
	Error     string        `json:"error,omitempty"`
}

func executeHealthCheck(_ context.Context, awsClient service.AWS) error {
	_, err := awsClient.S3get(config.Config.S3Bucket, config.Config.HealthCheckPath, nil)

	//if file exists, return ok
	if err == nil {
		return nil
	}

	//we have some kind of error. Normally we accept the 404 key not found because it means that we are able
	//to reach the endpoint without any issue.
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == s3.ErrCodeNoSuchKey {
			return nil
		}
	}

	return err
}

// HealthcheckHandler validates the s3 proxy dependencies and return a 500 if it is not ready to serve traffic
func HealthcheckHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	start := time.Now()
	httpRes := &HealthcheckResponse{}
	err := executeHealthCheck(req.Context(), service.NewClient(req.Context(), aws.String(config.Config.AwsRegion)))
	httpRes.S3Bucket.Time = time.Since(start)
	httpRes.S3Bucket.TimeHuman = httpRes.S3Bucket.Time.Milliseconds()
	if err == nil {
		httpRes.S3Bucket.Healthy = true
	} else {
		httpRes.S3Bucket.Error = err.Error()
	}
	//marshal response
	body, err := json.Marshal(httpRes)
	if err != nil {
		body = []byte(`{"error":"cannot marshal response"}`)
	}

	//if there was an error on unmarshaling or the end point is not healthy, then return an appropriate status code.
	if err != nil || !httpRes.S3Bucket.Healthy {
		w.WriteHeader(http.StatusInternalServerError)
	}

	//write final result
	_, _ = w.Write(body)
}
