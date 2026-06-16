package main

import (
	"testing"
	"time"
)

func TestEnvOrDefault(t *testing.T) {
	const key = "TEST_ENV_STRING"

	if got := envOrDefault(key, "fallback"); got != "fallback" {
		t.Errorf("unset: got %q, want %q", got, "fallback")
	}

	t.Setenv(key, "set")
	if got := envOrDefault(key, "fallback"); got != "set" {
		t.Errorf("set: got %q, want %q", got, "set")
	}

	t.Setenv(key, "")
	if got := envOrDefault(key, "fallback"); got != "" {
		t.Errorf("empty-but-present: got %q, want %q", got, "")
	}
}

func TestEnvIntOrDefault(t *testing.T) {
	const key = "TEST_ENV_INT"

	if got := envIntOrDefault(key, 42); got != 42 {
		t.Errorf("unset: got %d, want 42", got)
	}

	t.Setenv(key, "100")
	if got := envIntOrDefault(key, 42); got != 100 {
		t.Errorf("valid: got %d, want 100", got)
	}

	t.Setenv(key, "not-a-number")
	if got := envIntOrDefault(key, 42); got != 42 {
		t.Errorf("malformed: got %d, want fallback 42", got)
	}
}

func TestEnvDurationOrDefault(t *testing.T) {
	const key = "TEST_ENV_DURATION"
	def := 10 * time.Second

	if got := envDurationOrDefault(key, def); got != def {
		t.Errorf("unset: got %v, want %v", got, def)
	}

	t.Setenv(key, "30s")
	if got := envDurationOrDefault(key, def); got != 30*time.Second {
		t.Errorf("valid: got %v, want 30s", got)
	}

	t.Setenv(key, "soon")
	if got := envDurationOrDefault(key, def); got != def {
		t.Errorf("malformed: got %v, want fallback %v", got, def)
	}
}
