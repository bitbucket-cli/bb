package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func resetWebhookMocks() {
	webhookNewAPIClient = api.NewClient
}

func TestWebhookCommandStructure(t *testing.T) {
	if webhookCmd.Short == "" {
		t.Error("webhook command should have short description")
	}

	subCmds := webhookCmd.Commands()
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
			t.Errorf("Expected webhook subcommand %q not found", expected)
		}
	}
}

func TestWebhookCreateFlags(t *testing.T) {
	urlFlag := webhookCreateCmd.Flags().Lookup("url")
	if urlFlag == nil {
		t.Error("--url flag not found")
	}

	eventsFlag := webhookCreateCmd.Flags().Lookup("events")
	if eventsFlag == nil {
		t.Error("--events flag not found")
	}
}

func TestRunWebhookList_NoClient(t *testing.T) {
	defer resetWebhookMocks()

	webhookNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWebhookList(webhookListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWebhookList_InvalidRepo(t *testing.T) {
	defer resetWebhookMocks()

	webhookNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runWebhookList(webhookListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunWebhookList_APIError(t *testing.T) {
	defer resetWebhookMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookList(webhookListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunWebhookList_NoWebhooks(t *testing.T) {
	defer resetWebhookMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookList(webhookListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWebhookList_WithWebhooks(t *testing.T) {
	defer resetWebhookMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid":        "{123}",
					"url":         "https://example.com/webhook",
					"active":      true,
					"events":      []string{"repo:push"},
					"description": "Test webhook",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookList(webhookListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWebhookCreate_NoClient(t *testing.T) {
	defer resetWebhookMocks()

	webhookNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWebhookCreate(webhookCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWebhookCreate_Success(t *testing.T) {
	defer resetWebhookMocks()

	origURL := webhookURL
	webhookURL = "https://example.com/webhook"
	defer func() { webhookURL = origURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"uuid": "{new-webhook}"})
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookCreate(webhookCreateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWebhookCreate_APIError(t *testing.T) {
	defer resetWebhookMocks()

	origURL := webhookURL
	webhookURL = "https://example.com/webhook"
	defer func() { webhookURL = origURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid"})
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookCreate(webhookCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunWebhookView_NoClient(t *testing.T) {
	defer resetWebhookMocks()

	webhookNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWebhookView(webhookViewCmd, []string{"workspace/repo", "webhook-id"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWebhookView_Success(t *testing.T) {
	defer resetWebhookMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"uuid":        "{123}",
			"url":         "https://example.com/webhook",
			"active":      true,
			"events":      []string{"repo:push"},
			"description": "Test webhook",
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookView(webhookViewCmd, []string{"workspace/repo", "webhook-id"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWebhookDelete_NoClient(t *testing.T) {
	defer resetWebhookMocks()

	webhookNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWebhookDelete(webhookDeleteCmd, []string{"workspace/repo", "webhook-id"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWebhookDelete_Success(t *testing.T) {
	defer resetWebhookMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookDelete(webhookDeleteCmd, []string{"workspace/repo", "webhook-id"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWebhookDelete_APIError(t *testing.T) {
	defer resetWebhookMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	webhookNewAPIClient = mockAPIClientFactory(server)

	err := runWebhookDelete(webhookDeleteCmd, []string{"workspace/repo", "webhook-id"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}
