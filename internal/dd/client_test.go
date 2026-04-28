package dd

import (
	"context"
	"testing"
)

func TestResolveOptionsFromEnvironment(t *testing.T) {
	t.Setenv("DD_SITE", "https://app.us3.datadoghq.com/")
	t.Setenv("DD_API_KEY", "api")
	t.Setenv("DD_APP_KEY", "")
	t.Setenv("DD_APPLICATION_KEY", "app")

	opts := ResolveOptions(Options{})

	if opts.Site != "us3.datadoghq.com" {
		t.Fatalf("expected normalized site, got %q", opts.Site)
	}
	if opts.APIKey != "api" {
		t.Fatalf("expected API key from env")
	}
	if opts.AppKey != "app" {
		t.Fatalf("expected application key from env")
	}
}

func TestNewClientRequiresCredentials(t *testing.T) {
	t.Setenv("DD_API_KEY", "")
	t.Setenv("DD_APP_KEY", "")
	t.Setenv("DD_APPLICATION_KEY", "")
	t.Setenv("DD_ACCESS_TOKEN", "")

	_, err := NewClient(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected missing credentials error")
	}
}

func TestNewClientAcceptsAccessToken(t *testing.T) {
	t.Setenv("DD_API_KEY", "")
	t.Setenv("DD_APP_KEY", "")
	t.Setenv("DD_APPLICATION_KEY", "")
	t.Setenv("DD_ACCESS_TOKEN", "token")

	client, err := NewClient(context.Background(), Options{})
	if err != nil {
		t.Fatalf("expected access-token auth to be valid: %v", err)
	}
	if client.UsingAuth != "access_token" {
		t.Fatalf("expected access_token auth, got %q", client.UsingAuth)
	}
}
