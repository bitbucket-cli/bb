package bitbucket

import (
	"context"
	"github.com/bitbucket-cli/bb/pkg/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedOriginBoundaries(t *testing.T) {
	seen := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = true
		if r.Header.Get("Authorization") != "" {
			t.Error("redirect leaked credentials")
		}
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dummy" {
			t.Error("API request lacks credential")
		}
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer source.Close()
	client := &authClient{cred: auth.Credential{Token: "dummy"}, underlying: source.Client(), origin: requestOrigin(mustParseAPIURL(source.URL))}
	req, _ := http.NewRequestWithContext(context.Background(), "GET", destination.URL, nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("cross-origin request allowed")
	}
	if seen {
		t.Fatal("cross-origin destination contacted directly")
	}
	req, _ = http.NewRequest("GET", source.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !seen {
		t.Fatal("legitimate unauthenticated download redirect blocked")
	}
}

func TestAuthenticatedRedirectRejectsHTTPSDowngrade(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("downgrade contacted destination") }))
	defer destination.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer source.Close()
	client := &authClient{cred: auth.Credential{Token: "dummy"}, underlying: source.Client(), origin: requestOrigin(mustParseAPIURL(source.URL))}
	req, _ := http.NewRequest("GET", source.URL, nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("HTTPS downgrade allowed")
	}
}
