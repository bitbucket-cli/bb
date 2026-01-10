package main

import (
	"testing"
)

func TestRunCompletion(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "Bash completion",
			args:    []string{"bash"},
			wantErr: false,
		},
		{
			name:    "Zsh completion",
			args:    []string{"zsh"},
			wantErr: false,
		},
		{
			name:    "Fish completion",
			args:    []string{"fish"},
			wantErr: false,
		},
		{
			name:    "PowerShell completion",
			args:    []string{"powershell"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runCompletion(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runCompletion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
