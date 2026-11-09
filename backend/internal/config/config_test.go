package config

import "testing"

func TestRequiredDeploymentSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("PUBLIC_ORIGIN", "http://localhost:8080")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("STUDIO_TIME_ZONE", "")
	if _, err := Load(); err == nil {
		t.Fatal("timezone must not be guessed")
	}
	t.Setenv("STUDIO_TIME_ZONE", "Asia/Yekaterinburg")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PUBLIC_ORIGIN", "https://example.com/path")
	if _, err := Load(); err == nil {
		t.Fatal("origin with path accepted")
	}
}
