package tui

import "testing"

func TestSanitize(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{
		{"controls", "a\x00\x1b[31m\x7fb", "a[31mb"},
		{"preserves whitespace and runes", "a\n\r\tbé", "a\n\r\tbé"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.in); got != tt.want {
				t.Fatalf("sanitize() = %q, want %q", got, tt.want)
			}
		})
	}
}
