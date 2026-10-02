package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type sortFlags struct {
	dateAsc, dateDesc, fileAsc, fileDesc, numeric bool
}

func TestSortSetting(t *testing.T) {
	old := Config
	t.Cleanup(func() { Config = old })
	for sort, want := range map[string]sortFlags{
		"":           {fileAsc: true},
		"bogus":      {fileAsc: true},
		"filedesc":   {fileDesc: true},
		"dateasc":    {dateAsc: true},
		"datedesc":   {dateDesc: true},
		"numberasc":  {fileAsc: true, numeric: true},
		"numberdesc": {fileDesc: true, numeric: true},
	} {
		t.Setenv("SORT", sort)
		Setup()
		c := Config
		got := sortFlags{c.SortDateAsc, c.SortDateDesc,
			c.SortFileAsc, c.SortFileDesc, c.SortNumeric}
		assert.Equal(t, want, got, sort)
	}
}
