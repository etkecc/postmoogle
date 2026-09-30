package utils

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		text     string
		length   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"a long sentence", 6, "a long…"},
		{"çok güzel şeyler", 5, "çok g…"},
		{"ğüşıöç", 3, "ğüş…"},
		{"emoji 👍👍👍", 7, "emoji 👍…"},
		{"", 3, ""},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			if output := Truncate(test.text, test.length); output != test.expected {
				t.Errorf("expected %q, got %q", test.expected, output)
			}
		})
	}
}
