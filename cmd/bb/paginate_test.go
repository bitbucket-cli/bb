package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pageItem struct {
	ID int `json:"id"`
}

// TestFetchAllPages_FollowsNext proves the helper concatenates values across
// every "next" link rather than truncating at the first page.
func TestFetchAllPages_FollowsNext(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"values": []pageItem{{ID: 1}, {ID: 2}},
				"next":   server.URL + "/?page=2",
			})
		case "2":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"values": []pageItem{{ID: 3}, {ID: 4}},
				"next":   server.URL + "/?page=3",
			})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"values": []pageItem{{ID: 5}},
			})
		}
	}))
	defer server.Close()

	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	first, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	defer first.Body.Close()

	items, err := fetchAllPages[pageItem](context.Background(), client, first)
	if err != nil {
		t.Fatalf("fetchAllPages: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("expected 5 items across 3 pages, got %d (%v)", len(items), items)
	}
	for i, it := range items {
		if it.ID != i+1 {
			t.Errorf("item %d: expected id %d, got %d", i, i+1, it.ID)
		}
	}
}

// TestFetchAllPages_NextPageError surfaces a mid-pagination error instead of
// silently returning a truncated result.
func TestFetchAllPages_NextPageError(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"values": []pageItem{{ID: 1}},
			"next":   fmt.Sprintf("%s/?page=2", server.URL),
		})
	}))
	defer server.Close()

	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	first, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	defer first.Body.Close()

	if _, err := fetchAllPages[pageItem](context.Background(), client, first); err == nil {
		t.Fatal("expected an error when a later page returns 500, got nil")
	}
}
