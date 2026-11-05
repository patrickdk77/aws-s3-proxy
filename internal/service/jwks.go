package service

import (
	"context"
	"sync"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

var (
	jwksOnce sync.Once
	jwks     keyfunc.Keyfunc
	jwksErr  error
)

func JwksKeyfunc() (keyfunc.Keyfunc, error) {
	jwksOnce.Do(func() {
		c := config.Config
		jwks, jwksErr = keyfunc.NewDefaultCtx(context.Background(), []string{c.JwksURI})
	})
	return jwks, jwksErr
}
