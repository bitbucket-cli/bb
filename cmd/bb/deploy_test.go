package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func resetDeployMocks() {
	deployNewAPIClient = api.NewClient
	envType = "Test"
}

func TestDeployCommandStructure(t *testing.T) {
	if deployCmd.Short == "" {
		t.Error("deploy command should have short description")
	}

	subCmds := deployCmd.Commands()
	expectedCmds := []string{"list", "env"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected deploy subcommand %q not found", expected)
		}
	}
}

func TestDeployEnvCommandStructure(t *testing.T) {
	subCmds := deployEnvCmd.Commands()
	expectedCmds := []string{"list", "create", "delete"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected deploy env subcommand %q not found", expected)
		}
	}
}

func TestDeployEnvCreateFlags(t *testing.T) {
	typeFlag := deployEnvCreateCmd.Flags().Lookup("type")
	if typeFlag == nil {
		t.Error("--type flag not found")
	}
}

func TestRunDeployList_NoClient(t *testing.T) {
	defer resetDeployMocks()

	deployNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runDeployList(deployListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunDeployList_InvalidRepo(t *testing.T) {
	defer resetDeployMocks()

	deployNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runDeployList(deployListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunDeployList_APIError(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployList(deployListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunDeployList_NoDeployments(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployList(deployListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDeployList_WithDeployments(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid": "12345678-1234-1234-1234-123456789012",
					"environment": map[string]interface{}{
						"name": "production",
						"uuid": "env-uuid",
					},
					"release": map[string]interface{}{
						"name": "v1.0.0",
					},
					"state": map[string]interface{}{
						"name": "COMPLETED",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployList(deployListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDeployEnvList_NoClient(t *testing.T) {
	defer resetDeployMocks()

	deployNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runDeployEnvList(deployEnvListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunDeployEnvList_APIError(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvList(deployEnvListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunDeployEnvList_NoEnvironments(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvList(deployEnvListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDeployEnvList_WithEnvironments(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid": "env-uuid-1",
					"name": "production",
					"environment_type": map[string]interface{}{
						"name": "Production",
					},
				},
				map[string]interface{}{
					"uuid": "env-uuid-2",
					"name": "staging",
					"environment_type": map[string]interface{}{
						"name": "Staging",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvList(deployEnvListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDeployEnvCreate_NoClient(t *testing.T) {
	defer resetDeployMocks()

	deployNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runDeployEnvCreate(deployEnvCreateCmd, []string{"workspace/repo", "staging"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunDeployEnvCreate_Success(t *testing.T) {
	defer resetDeployMocks()

	envType = "Staging"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"uuid": "new-env-uuid"})
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvCreate(deployEnvCreateCmd, []string{"workspace/repo", "staging"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDeployEnvCreate_APIError(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid"})
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvCreate(deployEnvCreateCmd, []string{"workspace/repo", "staging"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunDeployEnvDelete_NoClient(t *testing.T) {
	defer resetDeployMocks()

	deployNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runDeployEnvDelete(deployEnvDeleteCmd, []string{"workspace/repo", "env-uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunDeployEnvDelete_Success(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvDelete(deployEnvDeleteCmd, []string{"workspace/repo", "env-uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDeployEnvDelete_APIError(t *testing.T) {
	defer resetDeployMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	deployNewAPIClient = mockAPIClientFactory(server)

	err := runDeployEnvDelete(deployEnvDeleteCmd, []string{"workspace/repo", "env-uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}
