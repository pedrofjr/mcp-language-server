package lsp

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestIsPermissionLikeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "os permission", err: os.ErrPermission, want: true},
		{name: "access denied string", err: errors.New("access is denied"), want: true},
		{name: "eperm string", err: errors.New("spawn EPERM"), want: true},
		{name: "other error", err: errors.New("file not found"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermissionLikeError(tt.err); got != tt.want {
				t.Fatalf("isPermissionLikeError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewProcessStartErrorIncludesHintsForPermissionErrors(t *testing.T) {
	err := newProcessStartError("LSP server", `C:\tmp\server.exe`, errors.New("operation not permitted"))
	message := err.Error()

	if !strings.Contains(message, "failed to start LSP server") {
		t.Fatalf("expected contextual prefix, got: %s", message)
	}
	if !strings.Contains(strings.ToLower(message), "unblock-file") {
		t.Fatalf("expected unblock hint, got: %s", message)
	}
}
