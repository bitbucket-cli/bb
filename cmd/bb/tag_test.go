package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ThomasRedstone/bb/internal/api"
)

func resetTagMocks() {
	tagNewAPIClient = api.NewClient
	tagTarget = ""
	tagMessage = ""
}

func TestTagCommandStructure(t *testing.T) {
	if tagCmd.Short == "" {
		t.Error("tag command should have short description")
	}

	subCmds := tagCmd.Commands()
	expectedCmds := []string{"list", "create", "view", "delete"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tag subcommand %q not found", expected)
		}
	}
}

func TestTagCreateFlags(t *testing.T) {
	targetFlag := tagCreateCmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Error("--target flag not found")
	}

	messageFlag := tagCreateCmd.Flags().Lookup("message")
	if messageFlag == nil {
		t.Error("--message flag not found")
	}
}

func TestRunTagList_NoClient(t *testing.T) {
	defer resetTagMocks()

	tagNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runTagList(tagListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunTagList_InvalidRepo(t *testing.T) {
	defer resetTagMocks()

	tagNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runTagList(tagListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunTagList_APIError(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagList(tagListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunTagList_NoTags(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagList(tagListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunTagList_WithTags(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"name": "v1.0.0",
					"target": map[string]interface{}{
						"hash":    "abc123def456",
						"date":    "2024-01-01T00:00:00Z",
						"message": "Initial release",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagList(tagListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunTagCreate_NoClient(t *testing.T) {
	defer resetTagMocks()

	tagNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runTagCreate(tagCreateCmd, []string{"workspace/repo", "v1.0.0"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunTagCreate_Success(t *testing.T) {
	defer resetTagMocks()

	tagTarget = "main"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "v1.0.0"})
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagCreate(tagCreateCmd, []string{"workspace/repo", "v1.0.0"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunTagCreate_WithMessage(t *testing.T) {
	defer resetTagMocks()

	tagTarget = "main"
	tagMessage = "Release 1.0.0"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "v1.0.0"})
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagCreate(tagCreateCmd, []string{"workspace/repo", "v1.0.0"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunTagCreate_APIError(t *testing.T) {
	defer resetTagMocks()

	tagTarget = "main"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid"})
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagCreate(tagCreateCmd, []string{"workspace/repo", "v1.0.0"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunTagView_NoClient(t *testing.T) {
	defer resetTagMocks()

	tagNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runTagView(tagViewCmd, []string{"workspace/repo", "v1.0.0"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunTagView_Success(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"name": "v1.0.0",
			"target": map[string]interface{}{
				"hash":    "abc123def456",
				"date":    "2024-01-01T00:00:00Z",
				"message": "Initial commit",
				"author": map[string]interface{}{
					"raw": "John Doe <john@example.com>",
				},
			},
			"message": "Release 1.0.0",
			"tagger": map[string]interface{}{
				"raw": "John Doe <john@example.com>",
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagView(tagViewCmd, []string{"workspace/repo", "v1.0.0"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunTagView_APIError(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagView(tagViewCmd, []string{"workspace/repo", "v1.0.0"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunTagDelete_NoClient(t *testing.T) {
	defer resetTagMocks()

	tagNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runTagDelete(tagDeleteCmd, []string{"workspace/repo", "v1.0.0"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunTagDelete_Success(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagDelete(tagDeleteCmd, []string{"workspace/repo", "v1.0.0"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunTagDelete_APIError(t *testing.T) {
	defer resetTagMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	tagNewAPIClient = mockAPIClientFactory(server)

	err := runTagDelete(tagDeleteCmd, []string{"workspace/repo", "v1.0.0"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}
