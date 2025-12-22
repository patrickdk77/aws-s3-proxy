package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-openapi/swag/typeutils"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/controllers"
	common "github.com/patrickdk77/aws-s3-proxy/internal/http"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ver    = "dev"
	commit string
	date   string
)

func main() {
	validateAwsConfigurations()

	httpMux := http.NewServeMux()

	if len(config.Config.MetricsPath) > 1 {
		httpMux.Handle(config.Config.MetricsPath, promhttp.Handler())
	}
	if len(config.Config.HealthCheckPath) > 1 {
		httpMux.HandleFunc(config.Config.HealthCheckPath, common.HealthcheckHandler)
	}
	if len(config.Config.VersionPath) > 1 {
		httpMux.HandleFunc(config.Config.VersionPath, func(w http.ResponseWriter, r *http.Request) {
			if len(commit) > 0 && len(date) > 0 {
				_, _ = fmt.Fprintf(w, "%s-%s (built at %s)\n", ver, commit, date)
				return
			}
			_, _ = fmt.Fprintln(w, ver)
		})
	}
	httpMux.Handle("/", common.WrapHandler(controllers.AwsS3))

	// Listen & Serve
	addr := net.JoinHostPort(config.Config.Host, config.Config.Port)
	log.Printf("[service] listening on %s", addr)

	s := &http.Server{
		ReadHeaderTimeout: 20 * time.Second,
		ReadTimeout:       config.Config.TimeoutRead,
		WriteTimeout:      config.Config.TimeoutWrite,
		Addr:              addr,
		Handler:           &slashFix{httpMux},
	}
	if (len(config.Config.SslCert) > 0) && (len(config.Config.SslKey) > 0) {
		log.Fatal(s.ListenAndServeTLS(config.Config.SslCert, config.Config.SslKey))
	} else {
		log.Fatal(s.ListenAndServe())
	}
}

func validateAwsConfigurations() {
	if len(os.Getenv("AWS_ACCESS_KEY_ID")) == 0 {
		log.Print("Not defined environment variable: AWS_ACCESS_KEY_ID")
	}
	if len(os.Getenv("AWS_SECRET_ACCESS_KEY")) == 0 {
		log.Print("Not defined environment variable: AWS_SECRET_ACCESS_KEY")
	}
	if len(os.Getenv("AWS_S3_BUCKET")) == 0 {
		log.Fatal("Missing required environment variable: AWS_S3_BUCKET")
	}
	if typeutils.IsZero(config.Config.AwsRegion) {
		config.Config.AwsRegion = "us-east-1"
		if region, err := service.GuessBucketRegion(config.Config.S3Bucket); err == nil {
			config.Config.AwsRegion = region
		}
	}
}

type slashFix struct {
	mux http.Handler
}

func (h *slashFix) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var pathBuilder strings.Builder
	slash := false
	for _, c := range r.URL.Path {
		if c == '/' {
			if !slash {
				pathBuilder.WriteRune(c)
			}
			slash = true
		} else {
			pathBuilder.WriteRune(c)
			slash = false
		}
	}
	r.URL.Path = pathBuilder.String()
	h.mux.ServeHTTP(w, r)
}
