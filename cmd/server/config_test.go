package main

import "testing"

func TestValidateConfig_ProductionRejectsWeakDefaults(t *testing.T) {
	cfg := config{
		AppEnv:        "production",
		DatabaseURL:   "postgres://x:y@db:5432/app?sslmode=disable",
		SessionSecret: "dev-secret-change-me",
		AdminPassword: "changeme123",
	}

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected production config with weak defaults to be rejected")
	}
}

func TestValidateConfig_ProductionAcceptsHardenedValues(t *testing.T) {
	cfg := config{
		AppEnv:        "production",
		DatabaseURL:   "postgres://x:y@db:5432/app?sslmode=require",
		SessionSecret: "0123456789abcdef0123456789abcdef",
		AdminPassword: "very-strong-admin-password",
	}

	if err := validateConfig(cfg); err != nil {
		t.Fatalf("expected hardened production config to pass, got: %v", err)
	}
}

func TestValidateConfig_DevelopmentAllowsDefaults(t *testing.T) {
	cfg := config{
		AppEnv:        "development",
		DatabaseURL:   "postgres://x:y@localhost:5432/app?sslmode=disable",
		SessionSecret: "dev-secret-change-me",
		AdminPassword: "changeme123",
	}

	if err := validateConfig(cfg); err != nil {
		t.Fatalf("expected development defaults to pass, got: %v", err)
	}
}
