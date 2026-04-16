package frpc

import (
	"io"
	"strings"
	"testing"

	"frp-helper/internal/model"
)

func TestResolveTargetAssetName(t *testing.T) {
	target, err := ResolveTarget("darwin", "arm64")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if got, want := target.AssetName("v0.68.0"), "frp_0.68.0_darwin_arm64.tar.gz"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMapRuntimeError(t *testing.T) {
	cases := map[string]string{
		"auth token invalid":            "frps authentication failed; check auth.token",
		"secret key mismatch":           "stcp authentication failed; check secretKey",
		"proxy name doesn't exist":      "target stcp service does not exist; check serverName",
		"address already in use":        "local bindPort is already in use",
		"dial tcp 1.2.3.4:7000 timeout": "failed to reach frps; check serverAddr/serverPort and network",
	}

	for input, want := range cases {
		if got := MapRuntimeError(input); got != want {
			t.Fatalf("input %q got %q want %q", input, got, want)
		}
	}
}

func TestRedactingWriter(t *testing.T) {
	var b strings.Builder
	writer := NewRedactingWriter([]io.Writer{&b}, []string{"secret"})
	if _, err := writer.Write([]byte("value=secret\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := b.String(); got != "value="+model.RedactedValue+"\n" {
		t.Fatalf("got %q", got)
	}
}
