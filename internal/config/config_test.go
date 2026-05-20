package config

import "testing"

func TestEnvBool(t *testing.T) {
	t.Setenv("SHCLOP_TEST_BOOL", "yes")
	if !envBool("SHCLOP_TEST_BOOL", false) {
		t.Fatal("expected yes to be true")
	}
	t.Setenv("SHCLOP_TEST_BOOL", "0")
	if envBool("SHCLOP_TEST_BOOL", true) {
		t.Fatal("expected 0 to be false")
	}
	t.Setenv("SHCLOP_TEST_BOOL", "maybe")
	if !envBool("SHCLOP_TEST_BOOL", true) {
		t.Fatal("expected fallback for unrecognized value")
	}
	if envBool("SHCLOP_TEST_BOOL", false) {
		t.Fatal("expected fallback false for unrecognized value")
	}
}
