package services

import (
	"strings"
	"testing"
)

func TestMaskKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want string
	}{
		{name: "empty", key: "  ", want: ""},
		{name: "short", key: "abc123", want: "••••••••"},
		{name: "standard", key: "sk-1234567890abcd", want: "sk-••••••••abcd"},
		{name: "long", key: "go-abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ12ZTU=", want: "go-••••••••ZTU="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskKey(tc.key); got != tc.want {
				t.Fatalf("maskKey(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestMaskKeyDoesNotLeakMiddle(t *testing.T) {
	key := "sk-SECRETMIDDLEVALUE-1234"
	masked := maskKey(key)
	if strings.Contains(masked, "SECRETMIDDLEVALUE") {
		t.Fatalf("masked key leaks middle: %q", masked)
	}
}
