package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	cache, err := os.MkdirTemp("", "mdhop-cli-cache-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CACHE_HOME", cache); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(cache)
	os.Exit(code)
}
