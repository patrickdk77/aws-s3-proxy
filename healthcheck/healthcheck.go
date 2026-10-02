package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	os.Exit(probe())
}

func probe() int {
	u := healthURL()
	if u == "" {
		return 0
	}
	insecure := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
	}
	c := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: insecure},
	}
	resp, err := c.Get(u) //nolint:gosec
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		return 2
	}
	return 0
}

func healthURL() string {
	host := os.Getenv("APP_HOST")
	if len(host) == 0 {
		host = "localhost"
	}
	port := os.Getenv("APP_PORT")
	if len(port) == 0 {
		port = "80"
	}
	proto := "http"
	ssl := os.Getenv("SSL_KEY_PATH")
	if len(ssl) > 0 {
		proto = "https"
	}
	health := os.Getenv("HEALTHCHECKER_PATH")
	if len(health) < 1 {
		health = os.Getenv("HEALTHCHECK_PATH")
	}
	if len(health) == 0 {
		return ""
	}
	return proto + "://" + net.JoinHostPort(host, port) + health
}
