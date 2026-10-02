package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type running struct {
	url    string
	cancel context.CancelFunc
	result chan error
}

func start(t *testing.T, grace time.Duration,
	h http.HandlerFunc) running {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &http.Server{Handler: h, ReadHeaderTimeout: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	res := make(chan error, 1)
	go func() {
		res <- serve(ctx, s, grace, func() error {
			return s.Serve(ln)
		})
	}()
	return running{"http://" + ln.Addr().String(), cancel, res}
}

func blocking(entered, release chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "done")
	}
}

func TestServeDrainsInFlight(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	r := start(t, 0, blocking(entered, release))
	got := make(chan string, 1)
	go func() {
		resp, err := http.Get(r.url)
		if err != nil {
			got <- err.Error()
			return
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		got <- string(b)
	}()
	<-entered
	r.cancel()
	select {
	case err := <-r.result:
		t.Fatalf("serve returned mid-request: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	assert.Equal(t, "done", <-got)
	select {
	case err := <-r.result:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the request")
	}
}

func TestServeGraceExpires(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	grace := 100 * time.Millisecond
	r := start(t, grace, blocking(entered, release))
	go func() {
		resp, err := http.Get(r.url)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered
	began := time.Now()
	r.cancel()
	select {
	case err := <-r.result:
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(began), grace)
	case <-time.After(5 * time.Second):
		t.Fatal("serve ignored the grace period")
	}
}

func TestServeListenError(t *testing.T) {
	want := errors.New("listen tcp :80: bind: address in use")
	err := serve(context.Background(), &http.Server{}, 0,
		func() error { return want })
	assert.Equal(t, want, err)
}

func TestServeRefusesAfterShutdown(t *testing.T) {
	r := start(t, time.Second, func(w http.ResponseWriter,
		_ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	resp, err := http.Get(r.url)
	if assert.NoError(t, err) {
		_ = resp.Body.Close()
	}
	r.cancel()
	assert.NoError(t, <-r.result)
	c := &http.Client{Transport: &http.Transport{}}
	_, err = c.Get(r.url)
	assert.Error(t, err)
}
