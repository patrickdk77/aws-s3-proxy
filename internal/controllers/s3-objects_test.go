package controllers

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortByS3objects1(t *testing.T) {
	expected := s3objects([]s3item{{file: "1"}, {file: "2"}, {file: "3"}})

	actual := s3objects([]s3item{{file: "3"}, {file: "1"}, {file: "2"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

func TestSortByS3objects2(t *testing.T) {
	expected := s3objects([]s3item{{file: "/10"}, {file: "/20"}, {file: "/101"}})

	actual := s3objects([]s3item{{file: "/20"}, {file: "/101"}, {file: "/10"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}

func TestSortByS3objects3(t *testing.T) {
	expected := s3objects([]s3item{{file: "/10/2"}, {file: "/101/1"}, {file: "/200/10"}})

	actual := s3objects([]s3item{{file: "/200/10"}, {file: "/10/2"}, {file: "/101/1"}})
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}
