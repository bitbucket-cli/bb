package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunSearch(t *testing.T) {
	defer func() { searchNewAPIClient = api.NewClient }()
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Get("search_query")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"content_match_count":3,"file":{"path":"a/b.go","commit":{"repository":{"full_name":"ws/repo"}}}}]}`))
	}))
	defer server.Close()

	searchNewAPIClient = mockAPIClientFactory(server)
	t.Setenv("BB_WORKSPACE", "myws")
	if err := runSearch(searchCmd, []string{"NewClient"}); err != nil {
		t.Fatalf("runSearch error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/workspaces/myws/search/code") {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "NewClient" {
		t.Errorf("search_query = %q, want NewClient", gotQuery)
	}
}
