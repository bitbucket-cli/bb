package main

import "testing"

func TestBranchingModelSource(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]interface{}
		want string
	}{
		{"inherited", map[string]interface{}{"branching_model": true, "default_merge_strategy": false}, "project (inherited)"},
		{"overridden", map[string]interface{}{"branching_model": false}, "repository (overrides project)"},
		{"missing", map[string]interface{}{"default_merge_strategy": true}, "unknown"},
		{"wrong type", map[string]interface{}{"branching_model": "yes"}, "unknown"},
	}
	for _, c := range cases {
		if got := branchingModelSource(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBranchOrUnset(t *testing.T) {
	named := &branchingModelBranch{Name: "main"}
	named.Branch = &struct {
		Name string `json:"name"`
	}{Name: "main"}

	cases := []struct {
		name string
		in   *branchingModelBranch
		want string
	}{
		{"absent", nil, "(not set)"},
		{"empty", &branchingModelBranch{}, "(not set)"},
		{"resolved", named, "main"},
		{"configured but missing", &branchingModelBranch{Name: "prod"}, "prod (branch not found)"},
		{"main branch missing", &branchingModelBranch{UseMainbranch: true}, "(main branch, not found)"},
	}
	for _, c := range cases {
		if got := branchOrUnset(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
