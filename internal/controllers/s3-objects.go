package controllers

import (
	"strings"
	"unicode"
	"time"

	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

type s3objects []s3item

type s3item struct {
  file string
  size int64
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
	}
	for idx := 0; idx < max; idx++ {
		ir := irs[idx]
		jr := jrs[idx]
		irl := unicode.ToLower(ir)
		jrl := unicode.ToLower(jr)

		if irl != jrl {
			if config.Config.SortDateAsc {
				return irl < jrl
			}
			if config.Config.SortDateDesc {
				return irl > jrl
			}
		}
		if ir != jr {
			if config.Config.SortDateAsc {
				return ir < jr
			}
			if config.Config.SortDateDesc {
				return ir > jr
			}
		}
	}
	return false
}
