package omnipascal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestReadMessage(t *testing.T) {
	payload := []byte(`{"type":"event","event":"syntaxDiag","body":{"file":"C:/tmp/test.pas","diagnostics":[]}}`)
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)

	got, err := readMessage(bufio.NewReader(bytes.NewBufferString(message)))
	if err != nil {
		t.Fatalf("readMessage() error = %v", err)
	}

	if string(got) != string(payload) {
		t.Fatalf("readMessage() = %s, want %s", got, payload)
	}
}

func TestReadMessageSkipsLeadingBlankLine(t *testing.T) {
	payload := []byte(`{"type":"response","request_seq":2,"success":true,"body":[]}`)
	message := fmt.Sprintf("\r\nContent-Length: %d\r\n\r\n%s", len(payload), payload)

	got, err := readMessage(bufio.NewReader(bytes.NewBufferString(message)))
	if err != nil {
		t.Fatalf("readMessage() error = %v", err)
	}

	if string(got) != string(payload) {
		t.Fatalf("readMessage() = %s, want %s", got, payload)
	}
}

func TestHandleEventCachesDiagnostics(t *testing.T) {
	client := &Client{
		syntaxDiagnostics:   make(map[string][]Diagnostic),
		semanticDiagnostics: make(map[string][]Diagnostic),
	}

	filePath := filepath.Join(t.TempDir(), "unit1.pas")
	syntaxBody, err := json.Marshal(DiagnosticsEvent{
		File: filePath,
		Diagnostics: []Diagnostic{{
			Start:    Point{Line: 1, Offset: 1},
			End:      Point{Line: 1, Offset: 5},
			Text:     "syntax issue",
			Severity: 1,
		}},
	})
	if err != nil {
		t.Fatalf("json.Marshal(syntax) error = %v", err)
	}

	semanticBody, err := json.Marshal(DiagnosticsEvent{
		File: filePath,
		Diagnostics: []Diagnostic{{
			Start:    Point{Line: 2, Offset: 3},
			End:      Point{Line: 2, Offset: 7},
			Text:     "semantic issue",
			Severity: 2,
		}},
	})
	if err != nil {
		t.Fatalf("json.Marshal(semantic) error = %v", err)
	}

	client.handleEvent(eventMessage{Event: "syntaxDiag", Body: syntaxBody})
	client.handleEvent(eventMessage{Event: "semanticDiag", Body: semanticBody})

	diagnostics := client.GetFileDiagnostics(filePath)
	if len(diagnostics) != 2 {
		t.Fatalf("len(GetFileDiagnostics()) = %d, want 2", len(diagnostics))
	}

	if diagnostics[0].Text != "syntax issue" {
		t.Fatalf("diagnostics[0].Text = %q, want %q", diagnostics[0].Text, "syntax issue")
	}

	if diagnostics[1].Text != "semantic issue" {
		t.Fatalf("diagnostics[1].Text = %q, want %q", diagnostics[1].Text, "semantic issue")
	}
}
