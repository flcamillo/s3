package main

import (
	"fmt"
	"testing"
	"time"
)

func TestRename(t *testing.T) {
	in := map[string][]string{
		"teste.txt":     {"#FN", "teste"},
		"XX/teste.txt":  {"#FN", "teste"},
		"/XX/teste.txt": {"#FN", "teste"},
		"c:\\teste.txt": {"#FN", "teste"},
	}
	for k, v := range in {
		n := parseName(k, v[0])
		if n != v[1] {
			t.Logf("[parseName] extract file name from {%s} => {%s} != {%s}", k, n, v[1])
			t.Fail()
		}
	}
	now := time.Now()
	in = map[string][]string{
		"teste.txt": {"#FN_#DD#DM#DY_#TH#TM#TS#FE", fmt.Sprintf("teste_%s_%s.txt", now.Format("02012006"), now.Format("150405"))},
		"abc.txt":   {"#FN_#DJ#FE", fmt.Sprintf("abc_%v.txt", now.YearDay())},
		"xyz.txt":   {"#FN_#YY#FE", fmt.Sprintf("xyz_%v.txt", now.Format("2006")[2:])},
	}
	for k, v := range in {
		n := parseName(k, v[0])
		if n != v[1] {
			t.Logf("[parseName] extract file name from {%s} => {%s} != {%s}", k, n, v[1])
			t.Fail()
		}
	}
}

func TestWildcardToRegexp(t *testing.T) {
	in := map[string]string{
		"*":        ".*",
		"*teste*":  ".*teste.*",
		"teste*":   "teste.*",
		"*teste":   ".*teste",
		"te*ste":   "te.*ste",
		"***teste": ".*teste",
	}
	for k, v := range in {
		n := wildCardToRegexp(k)
		if n != v {
			t.Logf("[wildCardToRegexp] conversion failed for {%s} => {%s} != {%s}", k, n, v)
			t.Fail()
		}
	}
}
