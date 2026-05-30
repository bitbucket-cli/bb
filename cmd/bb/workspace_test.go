package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ThomasRedstone/bb/internal/api"
)

func resetWorkspaceMocks() {
	workspaceNewAPIClient = api.NewClient
}

func TestWorkspaceCommandStructure(t *testing.T) {
	if workspaceCmd.Short == "" {
		t.Error("workspace command should have short description")
	}

	subCmds := workspaceCmd.Commands()
	expectedCmds := []string{"list", "view", "members", "projects"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected workspace subcommand %q not found", expected)
		}
	}
}

func TestRunWorkspaceList_NoClient(t *testing.T) {
	defer resetWorkspaceMocks()

	workspaceNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWorkspaceList(workspaceListCmd, []string{})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWorkspaceList_APIError(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceList(workspaceListCmd, []string{})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunWorkspaceList_NoWorkspaces(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceList(workspaceListCmd, []string{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWorkspaceList_WithWorkspaces(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid": "{ws-uuid-1}",
					"name": "My Workspace",
					"slug": "myworkspace",
					"type": "workspace",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceList(workspaceListCmd, []string{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWorkspaceView_NoClient(t *testing.T) {
	defer resetWorkspaceMocks()

	workspaceNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWorkspaceView(workspaceViewCmd, []string{"myworkspace"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWorkspaceView_APIError(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceView(workspaceViewCmd, []string{"myworkspace"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunWorkspaceView_Success(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"uuid":       "{ws-uuid}",
			"name":       "My Workspace",
			"slug":       "myworkspace",
			"type":       "workspace",
			"is_private": false,
			"created_on": "2024-01-01T00:00:00Z",
			"links": map[string]interface{}{
				"html": map[string]interface{}{
					"href": "https://bitbucket.org/myworkspace",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceView(workspaceViewCmd, []string{"myworkspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWorkspaceMembers_NoClient(t *testing.T) {
	defer resetWorkspaceMocks()

	workspaceNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWorkspaceMembers(workspaceMembersCmd, []string{"myworkspace"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWorkspaceMembers_APIError(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceMembers(workspaceMembersCmd, []string{"myworkspace"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunWorkspaceMembers_NoMembers(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceMembers(workspaceMembersCmd, []string{"myworkspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWorkspaceMembers_WithMembers(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"user": map[string]interface{}{
						"display_name": "John Doe",
						"nickname":     "johnd",
						"account_id":   "123456",
						"uuid":         "{user-uuid}",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceMembers(workspaceMembersCmd, []string{"myworkspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWorkspaceProjects_NoClient(t *testing.T) {
	defer resetWorkspaceMocks()

	workspaceNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runWorkspaceProjects(workspaceProjectsCmd, []string{"myworkspace"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunWorkspaceProjects_APIError(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceProjects(workspaceProjectsCmd, []string{"myworkspace"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunWorkspaceProjects_NoProjects(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceProjects(workspaceProjectsCmd, []string{"myworkspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunWorkspaceProjects_WithProjects(t *testing.T) {
	defer resetWorkspaceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"key":         "PROJ",
					"name":        "My Project",
					"description": "A sample project",
					"is_private":  true,
					"uuid":        "{proj-uuid}",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	workspaceNewAPIClient = mockAPIClientFactory(server)

	err := runWorkspaceProjects(workspaceProjectsCmd, []string{"myworkspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
