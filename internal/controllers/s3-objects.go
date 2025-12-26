package controllers

import (
	"strings"
	"time"
	"unicode"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

type s3objects []s3item

type s3item struct {
	file      string
	size      int64
	updatedAt time.Time
}

func (s s3objects) Len() int {
	return len(s)
}
func (s s3objects) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s s3objects) Less(i, j int) bool {
	if strings.Contains(s[i].file, "/") {
		if !strings.Contains(s[j].file, "/") {
			return true
		}
	} else {
		if strings.Contains(s[j].file, "/") {
			return false
		}
	}

	if config.Config.SortDateDesc && s[i].updatedAt != s[j].updatedAt {
		return s[i].updatedAt.After(s[j].updatedAt)
	}

	if config.Config.SortDateAsc && s[i].updatedAt != s[j].updatedAt {
		return s[i].updatedAt.Before(s[j].updatedAt)
	}

	irs := []rune(s[i].file)
	jrs := []rune(s[j].file)

	max := len(irs)
	if max > len(jrs) {
		max = len(jrs)
		if config.Config.SortNumeric {
			return config.Config.SortFileDesc
		}
	} else if max < len(jrs) {
		if config.Config.SortNumeric {
			return config.Config.SortFileAsc
		}
	}

	for idx := 0; idx < max; idx++ {
		ir := irs[idx]
		jr := jrs[idx]
		irl := unicode.ToLower(ir)
		jrl := unicode.ToLower(jr)

		if irl != jrl {
			if config.Config.SortFileAsc {
				return irl < jrl
			}
			if config.Config.SortFileDesc {
				return irl > jrl
			}
		}
		if ir != jr {
			if config.Config.SortFileAsc {
				return ir < jr
			}
			if config.Config.SortFileDesc {
				return ir > jr
			}
		}
	}
	if len(irs) < len(jrs) {
		return config.Config.SortFileAsc
	}
	if len(irs) > len(jrs) {
		return config.Config.SortFileDesc
	}
	return false
}
