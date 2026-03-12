package tools

import "testing"

func TestEncodeURIComponent(t *testing.T) {
	input := "Foo Bar\r\nBaz"
	want := "Foo%20Bar%0D%0ABaz"

	if got := encodeURIComponent(input); got != want {
		t.Fatalf("encodeURIComponent() = %q, want %q", got, want)
	}
}
