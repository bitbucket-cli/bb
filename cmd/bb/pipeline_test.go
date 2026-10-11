package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

// resetPipelineMocks restores default implementations
func resetPipelineMocks() {
	pipelineNewAPIClient = api.NewClient
	pipelineLimit = defaultPipelineLimit
}

func TestRunPipelineList_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineList_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineList(pipelineListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineView_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineView_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineView(pipelineViewCmd, []string{"invalid", "uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineTrigger_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineTrigger_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineStop_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineStop_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineStop(pipelineStopCmd, []string{"invalid", "uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestPipelineCommandStructure(t *testing.T) {
	if pipelineCmd.Short == "" {
		t.Error("pipeline command should have short description")
	}

	if pipelineListCmd.RunE == nil {
		t.Error("pipeline list should have RunE")
	}

	if pipelineViewCmd.RunE == nil {
		t.Error("pipeline view should have RunE")
	}

	if pipelineTriggerCmd.RunE == nil {
		t.Error("pipeline trigger should have RunE")
	}

	if pipelineStopCmd.RunE == nil {
		t.Error("pipeline stop should have RunE")
	}
}

func TestPipelineFlags(t *testing.T) {
	// Test --branch flag exists on trigger command
	branchFlag := pipelineTriggerCmd.Flags().Lookup("branch")
	if branchFlag == nil {
		t.Error("--branch flag not found on pipeline trigger command")
	}
}

func TestPipelineSubcommands(t *testing.T) {
	subCmds := pipelineCmd.Commands()
	expectedCmds := []string{"list", "view", "trigger", "stop"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pipeline subcommand %q not found", expected)
		}
	}
}

func TestPipelineAliases(t *testing.T) {
	if len(pipelineCmd.Aliases) == 0 {
		t.Error("pipeline command should have aliases")
	}

	found := false
	for _, alias := range pipelineCmd.Aliases {
		if alias == "pipelines" {
			found = true
			break
		}
	}
	if !found {
		t.Error("pipeline command should have 'pipelines' alias")
	}
}

func TestGetPipelineStatus(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *bitbucket.Pipeline
		want     string
	}{
		{
			name:     "nil state",
			pipeline: &bitbucket.Pipeline{State: nil},
			want:     "unknown",
		},
		{
			name: "empty state type",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{Type: ""},
			},
			want: "unknown",
		},
		{
			name: "state with type only",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{Type: "pending"},
			},
			want: "pending",
		},
		{
			name: "state with name in additional properties",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{
					Type:                 "some_type",
					AdditionalProperties: map[string]interface{}{"name": "IN_PROGRESS"},
				},
			},
			want: "IN_PROGRESS",
		},
		{
			name: "state with result",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{
					Type: "completed",
					AdditionalProperties: map[string]interface{}{
						"result": map[string]interface{}{"name": "SUCCESSFUL"},
					},
				},
			},
			want: "SUCCESSFUL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPipelineStatus(tt.pipeline)
			if got != tt.want {
				t.Errorf("getPipelineStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPipelineTarget(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *bitbucket.Pipeline
		want     string
	}{
		{
			name:     "nil target",
			pipeline: &bitbucket.Pipeline{Target: nil},
			want:     "",
		},
		{
			name: "target with ref_name",
			pipeline: &bitbucket.Pipeline{
				Target: &bitbucket.PipelineTarget{
					AdditionalProperties: map[string]interface{}{"ref_name": "main"},
				},
			},
			want: "main",
		},
		{
			name: "target without ref_name",
			pipeline: &bitbucket.Pipeline{
				Target: &bitbucket.PipelineTarget{
					AdditionalProperties: map[string]interface{}{},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPipelineTarget(tt.pipeline)
			if got != tt.want {
				t.Errorf("getPipelineTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "0s"},
		{30, "30s"},
		{59, "59s"},
		{60, "1m 0s"},
		{90, "1m 30s"},
		{3599, "59m 59s"},
		{3600, "1h 0m"},
		{3660, "1h 1m"},
		{7200, "2h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.seconds)
			if got != tt.want {
				t.Errorf("formatDuration(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestRunPipelineList_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineList_NoPipelines(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := bitbucket.PaginatedPipelines{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineList_WithPipelines(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		uuid := "{uuid}"
		createdOn := time.Now()
		duration := 120
		pipelines := []bitbucket.Pipeline{
			{
				BuildNumber:      &buildNum,
				Uuid:             &uuid,
				CreatedOn:        &createdOn,
				BuildSecondsUsed: &duration,
				State: &bitbucket.PipelineState{
					Type:                 "completed",
					AdditionalProperties: map[string]interface{}{"name": "SUCCESSFUL"},
				},
				Target: &bitbucket.PipelineTarget{
					AdditionalProperties: map[string]interface{}{"ref_name": "main"},
				},
			},
		}
		result := bitbucket.PaginatedPipelines{Values: &pipelines}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineList_RequestsNewestFirstWithDefaultPageLen(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if got := query.Get("sort"); got != "-created_on" {
			t.Errorf("sort query = %q, want -created_on", got)
		}
		if got := query.Get("pagelen"); got != "25" {
			t.Errorf("pagelen query = %q, want 25", got)
		}

		result := bitbucket.PaginatedPipelines{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineList_FollowsPaginationUntilLimit(t *testing.T) {
	defer resetPipelineMocks()
	pipelineLimit = 3

	requests := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++

		switch r.URL.Path {
		case "/repositories/workspace/repo/pipelines":
			query := r.URL.Query()
			if got := query.Get("sort"); got != "-created_on" {
				t.Errorf("sort query = %q, want -created_on", got)
			}
			if got := query.Get("pagelen"); got != "3" {
				t.Errorf("pagelen query = %q, want 3", got)
			}

			buildNum1 := 3
			buildNum2 := 2
			pipelines := []bitbucket.Pipeline{
				{BuildNumber: &buildNum1},
				{BuildNumber: &buildNum2},
			}
			next := server.URL + "/next"
			result := bitbucket.PaginatedPipelines{
				Values: &pipelines,
				Next:   &next,
			}
			json.NewEncoder(w).Encode(result)
		case "/next":
			buildNum := 1
			pipelines := []bitbucket.Pipeline{{BuildNumber: &buildNum}}
			result := bitbucket.PaginatedPipelines{Values: &pipelines}
			json.NewEncoder(w).Encode(result)
		default:
			t.Errorf("unexpected request path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if requests != 2 {
		t.Errorf("request count = %d, want 2", requests)
	}
}

func TestRunPipelineList_InvalidJSON(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPipelineView_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineView_Success(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		uuid := "{uuid}"
		createdOn := time.Now()
		completedOn := time.Now()
		duration := 120
		displayName := "Test User"
		pipeline := bitbucket.Pipeline{
			BuildNumber:      &buildNum,
			Uuid:             &uuid,
			CreatedOn:        &createdOn,
			CompletedOn:      &completedOn,
			BuildSecondsUsed: &duration,
			State: &bitbucket.PipelineState{
				Type:                 "completed",
				AdditionalProperties: map[string]interface{}{"name": "SUCCESSFUL"},
			},
			Target: &bitbucket.PipelineTarget{
				AdditionalProperties: map[string]interface{}{"ref_name": "main"},
			},
			Creator: &bitbucket.Account{
				DisplayName: &displayName,
			},
		}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineView_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		pipeline := bitbucket.Pipeline{BuildNumber: &buildNum}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	// UUID already has braces
	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "{uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineView_InvalidJSON(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPipelineView_JSONOutput(t *testing.T) {
	defer resetPipelineMocks()
	origJSONOutput := jsonOutput
	oldFormat := output.CurrentFormat
	output.SetFormat("json")
	defer func() { output.CurrentFormat = oldFormat }()
	jsonOutput = true
	defer func() { jsonOutput = origJSONOutput }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		pipeline := bitbucket.Pipeline{BuildNumber: &buildNum}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineTrigger_APIError(t *testing.T) {
	defer resetPipelineMocks()
	origBranch := pipelineBranch
	pipelineBranch = "main"
	defer func() { pipelineBranch = origBranch }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "bad request"})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineTrigger_Success(t *testing.T) {
	defer resetPipelineMocks()
	origBranch := pipelineBranch
	pipelineBranch = "main"
	defer func() { pipelineBranch = origBranch }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		buildNum := 42
		uuid := "{uuid}"
		pipeline := bitbucket.Pipeline{
			BuildNumber: &buildNum,
			Uuid:        &uuid,
		}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineTrigger_InvalidResponseJSON(t *testing.T) {
	defer resetPipelineMocks()
	origBranch := pipelineBranch
	pipelineBranch = "main"
	defer func() { pipelineBranch = origBranch }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPipelineStop_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineStop_Success(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineStop_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "{uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- Pipeline Steps Tests ---

func TestPipelineStepsSubcommands(t *testing.T) {
	subCmds := pipelineCmd.Commands()
	expectedCmds := []string{"list", "view", "trigger", "stop", "steps", "logs"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pipeline subcommand %q not found", expected)
		}
	}
}

func TestRunPipelineSteps_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineSteps_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineSteps(pipelineStepsCmd, []string{"invalid", "uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineSteps_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineSteps_NoSteps(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineSteps_FollowsPagination(t *testing.T) {
	defer resetPipelineMocks()

	const totalSteps = 11
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		makeStep := func(n int) map[string]interface{} {
			return map[string]interface{}{
				"uuid": fmt.Sprintf("{step-%d}", n),
				"name": fmt.Sprintf("Step %d", n),
				"state": map[string]interface{}{
					"name": "COMPLETED",
					"result": map[string]interface{}{
						"name": "SUCCESSFUL",
					},
				},
			}
		}

		if strings.Contains(r.URL.Path, "page2") {
			// Second (final) page: steps 11..totalSteps, no further "next".
			var values []interface{}
			for n := 11; n <= totalSteps; n++ {
				values = append(values, makeStep(n))
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"values": values})
			return
		}

		// First page: steps 1..10, plus a "next" link to page 2.
		var values []interface{}
		for n := 1; n <= 10; n++ {
			values = append(values, makeStep(n))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"values": values,
			"next":   server.URL + "/page2",
		})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	origFormat := output.CurrentFormat
	output.SetFormat("json")
	defer func() { output.CurrentFormat = origFormat }()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	runErr := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})

	w.Close()
	os.Stdout = origStdout
	captured, _ := io.ReadAll(r)

	if runErr != nil {
		t.Fatalf("Unexpected error: %v", runErr)
	}

	var steps []map[string]interface{}
	if err := json.Unmarshal(captured, &steps); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, captured)
	}
	if len(steps) != totalSteps {
		t.Fatalf("expected %d steps across pages, got %d", totalSteps, len(steps))
	}
}

func TestRunPipelineSteps_WithSteps(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		duration := 60
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid": "{step-uuid}",
					"name": "Build",
					"state": map[string]interface{}{
						"name": "COMPLETED",
						"result": map[string]interface{}{
							"name": "SUCCESSFUL",
						},
					},
					"duration_in_seconds": duration,
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineSteps_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "{uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- Pipeline Logs Tests ---

func TestRunPipelineLogs_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "pipeline-uuid", "step-uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineLogs_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineLogs(pipelineLogsCmd, []string{"invalid", "pipeline-uuid", "step-uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineLogs_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "pipeline-uuid", "step-uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineLogs_Success(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Build started...\nCompiling...\nBuild successful!"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "pipeline-uuid", "step-uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineLogs_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Log content"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "{pipeline-uuid}", "{step-uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- pull-request trigger -----------------------------------------------------

// Bitbucket accepts a pipeline_pullrequest_target ONLY on the trailing-slash
// path. Without it, it answers 400 "The request body contains invalid
// properties", which reads as a malformed body and is not one. Branch targets
// work on either path, so the quirk stays hidden until a PR trigger is tried -
// which is exactly why it is pinned here.
func TestRunPipelineTrigger_PullRequestUsesTrailingSlashPath(t *testing.T) {
	defer resetPipelineMocks()
	defer resetPipelineTriggerFlags()

	var triggerPath string
	var triggerBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pullrequests/") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"state": "OPEN",
				"source": map[string]interface{}{
					"branch": map[string]string{"name": "feat/thing"},
					"commit": map[string]string{"hash": "abc123"},
				},
				"destination": map[string]interface{}{
					"branch": map[string]string{"name": "main"},
					"commit": map[string]string{"hash": "def456"},
				},
			})
			return
		}
		triggerPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&triggerBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"build_number": 42})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	pipelinePullRequest = 197

	if err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(triggerPath, "/pipelines/") {
		t.Errorf("expected the trailing-slash pipelines path, got %q", triggerPath)
	}

	target, _ := triggerBody["target"].(map[string]interface{})
	if target["type"] != "pipeline_pullrequest_target" {
		t.Errorf("expected pipeline_pullrequest_target, got %v", target["type"])
	}
	// A branch target would run the same YAML but leave BITBUCKET_PR_ID unset,
	// failing any step that needs it.
	if target["source"] != "feat/thing" || target["destination"] != "main" {
		t.Errorf("source/destination not taken from the PR: %v / %v", target["source"], target["destination"])
	}
	pr, _ := target["pullrequest"].(map[string]interface{})
	// The API returns an integer id but rejects one here.
	if pr["id"] != "197" {
		t.Errorf("expected the PR id as the string \"197\", got %#v", pr["id"])
	}
}

func TestRunPipelineTrigger_PullRequestRejectsPattern(t *testing.T) {
	defer resetPipelineMocks()
	defer resetPipelineTriggerFlags()

	pipelineNewAPIClient = func() (*api.Client, error) { return &api.Client{}, nil }
	pipelinePullRequest = 197
	pipelinePattern = "deploy"

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Fatal("expected an error: Bitbucket cannot run custom pipelines for a pull request")
	}
	if !strings.Contains(err.Error(), "custom pipelines") {
		t.Errorf("error should explain why, got: %v", err)
	}
}

// A merged or declined PR has nothing to re-run, and Bitbucket's own error for
// it is not obvious.
func TestRunPipelineTrigger_PullRequestNotOpen(t *testing.T) {
	defer resetPipelineMocks()
	defer resetPipelineTriggerFlags()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"state": "MERGED",
			"source": map[string]interface{}{
				"branch": map[string]string{"name": "feat/thing"},
				"commit": map[string]string{"hash": "abc123"},
			},
		})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	pipelinePullRequest = 197

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil || !strings.Contains(err.Error(), "MERGED") {
		t.Errorf("expected an error naming the PR state, got: %v", err)
	}
}

func resetPipelineTriggerFlags() {
	pipelineBranch = ""
	pipelinePattern = ""
	pipelinePullRequest = 0
	pipelineVariables = nil
}
