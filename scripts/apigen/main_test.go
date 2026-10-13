package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestPreparePreservesNamedFieldsAndLiteralData(t *testing.T) {
	data := []byte(`{"openapi":"3.0.0","info":{"title":"API","version":"2","description":"prose"},"components":{"schemas":{"Example":{"description":"prose","properties":{"description":{"type":"string","description":"prose"},"summary":{"type":"string"},"example":{"type":"string"}},"default":{"description":"literal"},"enum":[{"summary":"literal"}],"example":{"description":"prose"}}}}}`)
	sum := sha256.Sum256(data)
	got, err := prepare(data, hex.EncodeToString(sum[:]), []byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	schema := doc["components"].(map[string]any)["schemas"].(map[string]any)["Example"].(map[string]any)
	if _, ok := schema["description"]; ok {
		t.Fatal("retained schema prose")
	}
	if _, ok := schema["example"]; ok {
		t.Fatal("retained example prose")
	}
	narrative := map[string]any{"x-atlassian-narrative": map[string]any{"documents": []any{map[string]any{"content": "upstream prose", "icon": "embedded image"}}}}
	strip(narrative, false)
	if len(narrative) != 0 {
		t.Fatal("retained narrative documentation")
	}
	props := schema["properties"].(map[string]any)
	for _, name := range []string{"description", "summary", "example"} {
		if props[name].(map[string]any)["type"] != "string" {
			t.Fatalf("lost API field %s", name)
		}
	}
	if _, ok := props["description"].(map[string]any)["description"]; ok {
		t.Fatal("retained field prose")
	}
	if schema["default"].(map[string]any)["description"] != "literal" || schema["enum"].([]any)[0].(map[string]any)["summary"] != "literal" {
		t.Fatal("changed literal API data")
	}
}

func TestPrepareRejectsChangedUpstreamBeforePatch(t *testing.T) {
	_, err := prepare([]byte(`{"openapi":"3.0.0"}`), strings.Repeat("0", 64), []byte(`invalid patch`))
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("wanted checksum failure, got %v", err)
	}
}

func TestCompatibilityEscapedPathsAndFailures(t *testing.T) {
	doc := map[string]any{"paths": map[string]any{"/old~route": map[string]any{"operationId": "old"}}}
	patch := []byte(`[{"op":"replace","path":"/paths/~1old~0route/operationId","value":"stable"},{"op":"add","path":"/paths/~1new","value":{}},{"op":"remove","path":"/paths/~1old~0route"}]`)
	if err := applyPatch(doc, patch); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc, map[string]any{"paths": map[string]any{"/new": map[string]any{}}}) {
		t.Fatalf("unexpected patch result %v", doc)
	}
	for _, patch := range []string{
		`[{"op":"remove","path":"/missing"}]`,
		`[{"op":"replace","path":"/missing/key","value":0}]`,
		`[{"op":"move","path":"/paths"}]`,
		`[{"op":"add","path":"invalid","value":0}]`,
	} {
		if err := applyPatch(doc, []byte(patch)); err == nil {
			t.Fatalf("accepted invalid patch %s", patch)
		}
	}
}

func TestDownloadHTTPSStatusAndDowngrade(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/schema":
			_, _ = w.Write([]byte(`{"openapi":"3.0.0"}`))
		case "/redirect":
			http.Redirect(w, r, "http://example.invalid/schema", http.StatusFound)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	got, err := download(context.Background(), server.URL+"/schema", server.Client())
	if err != nil || string(got) != `{"openapi":"3.0.0"}` {
		t.Fatalf("download got %q, %v", got, err)
	}
	for _, path := range []string{"/redirect", "/missing"} {
		if _, err := download(context.Background(), server.URL+path, server.Client()); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	if _, err := download(context.Background(), "http://example.invalid/schema", server.Client()); err == nil {
		t.Fatal("accepted plaintext URL")
	}
}
