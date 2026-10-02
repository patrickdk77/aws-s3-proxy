package controllers

import (
	"sort"
	"testing"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func sortedWith(t *testing.T, setting string,
	names ...string) []string {
	t.Helper()
	old := config.Config
	t.Cleanup(func() { config.Config = old })
	t.Setenv("SORT", setting)
	config.Setup()
	items := s3objects{}
	for _, n := range names {
		items = append(items, s3item{file: n})
	}
	sort.Sort(items)
	out := []string{}
	for _, i := range items {
		out = append(out, i.file)
	}
	return out
}

func TestSortNumberDesc(t *testing.T) {
	got := sortedWith(t, "numberdesc",
		"2", "10", "1", "30", "3", "dir/")
	want := []string{"dir/", "30", "10", "3", "2", "1"}
	assert.Equal(t, want, got)
}

func TestSortNumberAsc(t *testing.T) {
	got := sortedWith(t, "numberasc",
		"2", "10", "1", "30", "3", "dir/")
	want := []string{"dir/", "1", "2", "3", "10", "30"}
	assert.Equal(t, want, got)
}

func TestSortFileDescNotNumeric(t *testing.T) {
	got := sortedWith(t, "filedesc", "2", "10", "1")
	assert.Equal(t, []string{"2", "10", "1"}, got)
}
