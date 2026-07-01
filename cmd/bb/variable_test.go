package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetVariableMocks() {
	variableNewAPIClient = api.NewClient
	variableSecured = false
}

func TestVariableCommandStructure(t *testing.T) {
	if variableCmd.Short == "" {
		t.Error("variable command should have a short description")
	}
	expected := []string{"list", "set", "delete"}
	for _, name := range expected {
		found := false
		for _, c := range variableCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected variable subcommand %q not found", name)
		}
	}
}

func TestRunVariableList(t *testing.T) {
	defer resetVariableMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"uuid":"{u1}","key":"FOO","value":"bar","secured":false},{"uuid":"{u2}","key":"SECRET","secured":true}]}`))
	}))
	defer server.Close()

	variableNewAPIClient = mockAPIClientFactory(server)
	if err := runVariableList(variableListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runVariableList error: %v", err)
	}
}

func TestRunVariableSet_Create(t *testing.T) {
	defer resetVariableMocks()

	var sawPost bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"values":[]}`)) // no existing variable -> create
		case http.MethodPost:
			sawPost = true
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"uuid":"{new}","key":"FOO","value":"bar"}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	variableNewAPIClient = mockAPIClientFactory(server)
	if err := runVariableSet(variableSetCmd, []string{"myws/myrepo", "FOO", "bar"}); err != nil {
		t.Fatalf("runVariableSet error: %v", err)
	}
	if !sawPost {
		t.Error("expected a POST to create the new variable")
	}
}

func TestRunVariableSet_Update(t *testing.T) {
	defer resetVariableMocks()

	var sawPut bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"values":[{"uuid":"{u1}","key":"FOO","value":"old","secured":false}]}`))
		case http.MethodPut:
			sawPut = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"uuid":"{u1}","key":"FOO","value":"new"}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	variableNewAPIClient = mockAPIClientFactory(server)
	if err := runVariableSet(variableSetCmd, []string{"myws/myrepo", "FOO", "new"}); err != nil {
		t.Fatalf("runVariableSet error: %v", err)
	}
	if !sawPut {
		t.Error("expected a PUT to update the existing variable")
	}
}

func TestRunVariableDelete(t *testing.T) {
	defer resetVariableMocks()

	var sawDelete bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"values":[{"uuid":"{u1}","key":"FOO","value":"bar"}]}`))
		case http.MethodDelete:
			sawDelete = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	variableNewAPIClient = mockAPIClientFactory(server)
	if err := runVariableDelete(variableDeleteCmd, []string{"myws/myrepo", "FOO"}); err != nil {
		t.Fatalf("runVariableDelete error: %v", err)
	}
	if !sawDelete {
		t.Error("expected a DELETE call")
	}
}

func TestRunVariableDelete_NotFound(t *testing.T) {
	defer resetVariableMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[]}`))
	}))
	defer server.Close()

	variableNewAPIClient = mockAPIClientFactory(server)
	err := runVariableDelete(variableDeleteCmd, []string{"myws/myrepo", "MISSING"})
	if err == nil {
		t.Error("expected error when deleting a non-existent variable")
	}
}

func TestRunVariableList_NoClient(t *testing.T) {
	defer resetVariableMocks()

	variableNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}
	if err := runVariableList(variableListCmd, []string{"myws/myrepo"}); err == nil {
		t.Error("expected error when client creation fails")
	}
}
