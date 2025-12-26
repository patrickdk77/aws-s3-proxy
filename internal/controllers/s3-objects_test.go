package controllers

import (
	"sort"
	"testing"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

// Test normal file asc sort
func TestSortByS3objects1(t *testing.T) {
	config.Config.SortFileAsc = true
	config.Config.SortNumeric = false
	expected := s3objects([]s3item{{file: "1"}, {file: "2"}, {file: "3"}})

	actual := s3objects([]s3item{{file: "3"}, {file: "1"}, {file: "2"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

// Test normal file asc sort
func TestSortByS3objects2(t *testing.T) {
	config.Config.SortFileAsc = true
	config.Config.SortNumeric = false
	expected := s3objects([]s3item{{file: "/10"}, {file: "/101"}, {file: "/20"}})

	actual := s3objects([]s3item{{file: "/20"}, {file: "/101"}, {file: "/10"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

// Test normal file asc sort
func TestSortByS3objects3(t *testing.T) {
	config.Config.SortFileAsc = true
	config.Config.SortNumeric = false
	expected := s3objects([]s3item{{file: "/10/2"}, {file: "/101/1"}, {file: "/200/10"}})

	actual := s3objects([]s3item{{file: "/200/10"}, {file: "/10/2"}, {file: "/101/1"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

// Test numeric file asc sort
func TestSortByS3objects11(t *testing.T) {
	config.Config.SortFileAsc = true
	config.Config.SortNumeric = true
	expected := s3objects([]s3item{{file: "1"}, {file: "2"}, {file: "3"}})

	actual := s3objects([]s3item{{file: "3"}, {file: "1"}, {file: "2"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

// Test numeric file asc sort
func TestSortByS3objects12(t *testing.T) {
	config.Config.SortFileAsc = true
	config.Config.SortNumeric = true
	expected := s3objects([]s3item{{file: "/10"}, {file: "/20"}, {file: "/101"}})

	actual := s3objects([]s3item{{file: "/20"}, {file: "/101"}, {file: "/10"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

// Test numeric file asc sort
func TestSortByS3objects13(t *testing.T) {
	config.Config.SortFileAsc = true
	config.Config.SortNumeric = true
	expected := s3objects([]s3item{{file: "/10/2"}, {file: "/101/1"}, {file: "/200/10"}})

	actual := s3objects([]s3item{{file: "/200/10"}, {file: "/10/2"}, {file: "/101/1"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}
