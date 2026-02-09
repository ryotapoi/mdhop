package main

import (
	"strings"
	"testing"
)

func TestRunBuild_InvalidFlag(t *testing.T) {
	err := runBuild([]string{"--invalid"})
	if err == nil {
		t.Error("expected error for invalid flag")
	}
}

func TestRunResolve_MissingFrom(t *testing.T) {
	err := runResolve([]string{"--link", "[[X]]"})
	if err == nil || !strings.Contains(err.Error(), "--from is required") {
		t.Errorf("expected --from required error, got: %v", err)
	}
}

func TestRunResolve_MissingLink(t *testing.T) {
	err := runResolve([]string{"--from", "A.md"})
	if err == nil || !strings.Contains(err.Error(), "--link is required") {
		t.Errorf("expected --link required error, got: %v", err)
	}
}

func TestRunResolve_InvalidFormat(t *testing.T) {
	err := runResolve([]string{"--from", "A.md", "--link", "[[X]]", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}

func TestRunResolve_InvalidField(t *testing.T) {
	err := runResolve([]string{"--from", "A.md", "--link", "[[X]]", "--fields", "type,invalid"})
	if err == nil || !strings.Contains(err.Error(), "unknown resolve field") {
		t.Errorf("expected unknown field error, got: %v", err)
	}
}

func TestRunQuery_InvalidFormat(t *testing.T) {
	err := runQuery([]string{"--file", "A.md", "--format", "yaml"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected invalid format error, got: %v", err)
	}
}
