package config

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupWith(t *testing.T, env map[string]string) string {
	t.Helper()
	old := Config
	t.Cleanup(func() { Config = old })
	for k, v := range env {
		t.Setenv(k, v)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	Setup()
	return buf.String()
}

func TestEnvValidValues(t *testing.T) {
	logs := setupWith(t, map[string]string{
		"SPA":                  "true",
		"CACHE_SIZE":           "8",
		"MAX_IDLE_CONNECTIONS": "300",
		"GET_TIMEOUT":          "30",
	})
	assert.True(t, Config.SPA)
	assert.Equal(t, int64(8<<20), Config.CacheSize)
	assert.Equal(t, 300, Config.MaxIdleConns)
	assert.Equal(t, 30*time.Second, Config.TimeoutWrite)
	assert.NotContains(t, logs, "ignoring")
}

func TestEnvUnsetKeepsDefaults(t *testing.T) {
	logs := setupWith(t, map[string]string{
		"CACHE_SIZE":       "",
		"CONTENT_ENCODING": "",
	})
	assert.Equal(t, int64(0), Config.CacheSize)
	assert.True(t, Config.ContentEncoding)
	assert.NotContains(t, logs, "ignoring")
}

func TestEnvInvalidIntLogged(t *testing.T) {
	logs := setupWith(t, map[string]string{"CACHE_SIZE": "1G"})
	assert.Equal(t, int64(0), Config.CacheSize)
	assert.Contains(t, logs, "ignoring invalid CACHE_SIZE")
}

func TestEnvInvalidBoolLogged(t *testing.T) {
	logs := setupWith(t, map[string]string{"SPA": "yes"})
	assert.False(t, Config.SPA)
	assert.Contains(t, logs, "ignoring invalid SPA")
}

func TestEnvOutOfRangeLogged(t *testing.T) {
	logs := setupWith(t, map[string]string{
		"MAX_IDLE_CONNECTIONS": "40000",
	})
	assert.Equal(t, 150, Config.MaxIdleConns)
	assert.Contains(t, logs,
		"ignoring invalid MAX_IDLE_CONNECTIONS")
}

func TestEnvInvalidDurationLogged(t *testing.T) {
	logs := setupWith(t, map[string]string{"GET_TIMEOUT": "10m"})
	assert.Equal(t, 600*time.Second, Config.TimeoutWrite)
	assert.Contains(t, logs, "ignoring invalid GET_TIMEOUT")
}

func TestEnvMaxFileSizeDefault(t *testing.T) {
	setupWith(t, map[string]string{
		"CACHE_SIZE":          "10",
		"CACHE_MAX_FILE_SIZE": "",
	})
	assert.Equal(t, int64(10<<20)/4, Config.CacheMaxFileSize)
}

func TestEnvUnknownSortLogged(t *testing.T) {
	logs := setupWith(t, map[string]string{"SORT": "size"})
	assert.True(t, Config.SortFileAsc)
	assert.Contains(t, logs, "ignoring unknown SORT")
}

func TestEnvFileAscNotLogged(t *testing.T) {
	logs := setupWith(t, map[string]string{"SORT": "fileasc"})
	assert.True(t, Config.SortFileAsc)
	assert.NotContains(t, logs, "ignoring")
}
