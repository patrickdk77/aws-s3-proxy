package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	host := os.Getenv("APP_HOST")
	if len(host) == 0 {
		host = "localhost"
	}
	port := os.Getenv("APP_PORT")
	if len(host) == 0 {
		port = "80"
	}
	proto := "http"
	ssl := os.Getenv("SSL_KEY_PATH")
	if len(ssl) > 0 {
		proto = "https"
	}
	health := os.Getenv("HEALTHCHECKER_PATH")
	if len(health)<1 {
		health = os.Getenv("HEALTHCHECK_PATH")
	}
	if len(health)>0 {
		resp, err := http.Get(fmt.Sprintf("%s://%s:%s%s", proto, host, port, health))
		if err != nil {
			os.Exit(1)
		}
		if resp.StatusCode != 200 {
			os.Exit(2)
		}
	}
	os.Exit(0)
}
