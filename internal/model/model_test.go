package model

import (
	"strings"
	"testing"
)

func TestServiceKeyAndAccessCommand(t *testing.T) {
	t.Run("service key resolves global user", func(t *testing.T) {
		svc := Service{ServerName: "ubuntu_ssh"}
		if got, want := svc.Key("ops"), "ops:ubuntu_ssh"; got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("ssh access command falls back to placeholder", func(t *testing.T) {
		svc := Service{ProtocolHint: "ssh", BindPort: 6000}
		if got, want := svc.AccessCommand(), "ssh -p 6000 <username>@127.0.0.1"; got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

func TestManifestValidateDetectsConflicts(t *testing.T) {
	manifest := Manifest{
		ServerAddr: "frps.example.com",
		ServerPort: 7000,
		AuthToken:  "token",
		User:       "ops",
		Services: []Service{
			{Name: "a", ServerName: "same", SecretKey: "one", BindPort: 6000},
			{Name: "b", ServerName: "same", SecretKey: "two", BindPort: 6000},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	text := err.Error()
	if !strings.Contains(text, "duplicate service-key") {
		t.Fatalf("expected duplicate service-key error, got %q", text)
	}
	if !strings.Contains(text, "duplicate bindPort") {
		t.Fatalf("expected duplicate bindPort error, got %q", text)
	}
}

func TestRenderTOMLIncludesEnabledVisitorsOnly(t *testing.T) {
	manifest := Manifest{
		ServerAddr: "frps.example.com",
		ServerPort: 7000,
		AuthToken:  "token",
		User:       "ops",
		Services: []Service{
			{Name: "ubuntu", ServerName: "ubuntu_ssh", SecretKey: "secret", BindPort: 6000, ProtocolHint: "ssh"},
			{Name: "disabled", ServerName: "disabled_ssh", SecretKey: "secret2", BindPort: 6001, Disabled: true},
		},
	}

	text := RenderTOML(manifest)
	if !strings.Contains(text, `auth.token = "token"`) {
		t.Fatalf("expected auth token in toml: %s", text)
	}
	if !strings.Contains(text, `serverUser = "ops"`) {
		t.Fatalf("expected serverUser in toml: %s", text)
	}
	if strings.Contains(text, "disabled_ssh") {
		t.Fatalf("disabled service should not be rendered: %s", text)
	}
}

func TestSanitize(t *testing.T) {
	got := Sanitize("token abc and key def", []string{"abc", "def"})
	if want := "token " + RedactedValue + " and key " + RedactedValue; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
