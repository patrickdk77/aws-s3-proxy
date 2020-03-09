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

// HealthcheckRespose struct builds the healthcheck endpoint response
type HealthcheckRespose struct {
	S3Bucket healthcheck `json:"s3_bucket"`
}

// Healthcheck wraps the content of each service dependency
type healthcheck struct {
	Healthy bool          `json:"healthy"`
	Time    time.Duration `json:"time_ns"`
	Error   string        `json:"error"`
}

// Healthcheck validates the s3 proxy dependencies and return a 500 if it is not ready to serve traffic
func Healthcheck(w http.ResponseWriter, r *http.Request) {
	res := &HealthcheckRespose{
		S3Bucket: healthcheck{
			Healthy: false,
			Time:    0,
			Error:   "",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := res.checkS3Bucket(); err != nil {
		res.S3Bucket.Error = err.Error()
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				res.S3Bucket.Healthy = true
				res.S3Bucket.Error = ""
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	js, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"cannot marshal response"}`))
		return
	}
	res.S3Bucket.Healthy = true
	w.Write(js)
}

// This function saves the time it took another function to complete
func timeTrack(start time.Time, timer *time.Duration) {
	*timer = time.Since(start)
}

// Check S3 bucket connectivity
func (h *HealthcheckRespose) checkS3Bucket() error {
	defer timeTrack(time.Now(), &(h.S3Bucket.Time))
	bucket := config.Config.S3Bucket
	key := "/healthz"

	client := service.NewClient(context.Background(), aws.String(config.Config.AwsRegion))
	if _, err := client.S3get(bucket, key, nil); err != nil {
		h.S3Bucket.Error = err.Error()
		return err
	}

	h.S3Bucket.Healthy = true
	return nil
}