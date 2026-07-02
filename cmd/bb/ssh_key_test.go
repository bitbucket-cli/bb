package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunSSHKeyList(t *testing.T) {
	defer func() { sshKeyNewAPIClient = api.NewClient }()
	var keysPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/user") {
			w.Write([]byte(`{"account_id":"acc123"}`))
			return
		}
		keysPath = r.URL.Path
		w.Write([]byte(`{"values":[{"uuid":"{k}","key":"ssh-ed25519 AAAA","label":"ci"}]}`))
	}))
	defer srv.Close()
	sshKeyNewAPIClient = mockAPIClientFactory(srv)
	if err := runSSHKeyList(sshKeyListCmd, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(keysPath, "/users/acc123/ssh-keys") {
		t.Errorf("keysPath=%q", keysPath)
	}
}
