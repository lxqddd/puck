package main

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	securityHeaders(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app.html", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}

	csp := recorder.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"'sha256-",
		"style-src 'self' 'unsafe-inline'",
		"object-src 'none'",
		"base-uri 'none'",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	if got := recorder.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
}

func TestCSPHashMatchesMascotBootScript(t *testing.T) {
	html, err := assets.ReadFile("frontend/dist/mascot.html")
	if err != nil {
		t.Fatalf("read mascot.html: %v", err)
	}
	match := regexp.MustCompile(`<script>([\s\S]*?)</script>`).FindSubmatch(html)
	if len(match) != 2 {
		t.Fatal("mascot inline boot script not found")
	}
	sum := sha256.Sum256(match[1])
	want := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	if !strings.Contains(contentSecurityPolicy, want) {
		t.Fatalf("CSP hash does not match mascot boot script: want %s", want)
	}
}
