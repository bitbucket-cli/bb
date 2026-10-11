package bitbucket

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	generated "github.com/bitbucket-cli/bb/generated/bitbucket"
)

// newTestClient builds a Client whose embedded generated client points at the
// given server (no auth layer), mirroring how the cmd tests mock the API.
func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	gen, err := generated.NewClient(serverURL)
	if err != nil {
		t.Fatalf("generated.NewClient: %v", err)
	}
	return &Client{Client: gen}
}

func TestRequest_JSONBodyAndServer(t *testing.T) {
	var gotMethod, gotPath, gotCT, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	resp, err := client.Request(context.Background(), http.MethodPost, "/foo/bar", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	resp.Body.Close()

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/foo/bar" {
		t.Errorf("path = %q, want /foo/bar", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCT)
	}
	if !strings.Contains(gotBody, `"k":"v"`) {
		t.Errorf("body = %q, want it to contain the JSON", gotBody)
	}
}

func TestRequest_NoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "" {
			t.Errorf("expected no content-type for body-less request, got %q", ct)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	resp, err := client.Request(context.Background(), http.MethodDelete, "/x", nil)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	resp.Body.Close()
}

func TestRequestRaw(t *testing.T) {
	var gotMethod, gotCT, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	resp, err := client.RequestRaw(context.Background(), http.MethodPut, "/raw", "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("RequestRaw error: %v", err)
	}
	resp.Body.Close()

	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotCT != "text/plain" {
		t.Errorf("content-type = %q, want text/plain", gotCT)
	}
	if gotBody != "payload" {
		t.Errorf("body = %q, want payload", gotBody)
	}
}
