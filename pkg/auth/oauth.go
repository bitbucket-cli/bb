package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	authorizationURL = "https://bitbucket.org/site/oauth2/authorize"
	defaultTokenURL  = "https://bitbucket.org/site/oauth2/access_token"
	callbackPort     = 34106
	callbackPath     = "/callback"
)

// Mockable functions for testing
var (
	tokenURL      = defaultTokenURL
	openBrowserFn = openBrowserReal
	httpClientFn  = func() *http.Client { return &http.Client{Timeout: 30 * time.Second} }
	randRead      = rand.Read
)

// OAuthConfig holds OAuth configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// scopePresets map a privilege LEVEL to the OAuth scopes requested at login.
//
// Bitbucket validates the requested scopes against the OAuth consumer — login
// fails if you ask for a scope the consumer wasn't granted — but it does NOT
// narrow the issued token: the access token always carries the consumer's full
// configured scopes, regardless of what we request. So a preset is a fail-fast
// assertion of the capability level you expect your consumer to provide, not a
// way to restrict the token. The real least-privilege lever is the consumer's
// own permissions.
var scopePresets = map[string][]string{
	"read": {
		"account", "repository", "pullrequest", "pipeline",
	},
	"write": {
		"account",
		"repository", "repository:write",
		"pullrequest", "pullrequest:write",
		"pipeline", "pipeline:write",
	},
	"admin": {
		"account",
		"repository", "repository:write", "repository:admin",
		"pullrequest", "pullrequest:write",
		"pipeline", "pipeline:write",
	},
}

// DefaultScopeLevel is the preset requested when none is specified.
const DefaultScopeLevel = "write"

// ScopePresetNames returns the preset names in increasing-privilege order.
func ScopePresetNames() []string {
	return []string{"read", "write", "admin"}
}

// DefaultScopes returns the OAuth scopes for the default level.
func DefaultScopes() []string {
	return append([]string(nil), scopePresets[DefaultScopeLevel]...)
}

// ResolveScopes turns a --scopes spec into a concrete scope list. An empty spec
// yields the default level. A known preset name (read/write/admin) expands to
// its set; any other value is treated as an explicit comma/space-separated
// scope list, so power users can request an exact set.
func ResolveScopes(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = DefaultScopeLevel
	}
	if preset, ok := scopePresets[spec]; ok {
		return append([]string(nil), preset...), nil
	}
	fields := strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' })
	if len(fields) == 0 {
		return nil, fmt.Errorf("no scopes specified in %q", spec)
	}
	return fields, nil
}

// Login performs the OAuth 2.0 authorization code flow
func Login(cfg OAuthConfig) (*TokenData, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("OAuth client credentials not configured")
	}

	if len(cfg.Scopes) == 0 {
		cfg.Scopes = DefaultScopes()
	}

	// Generate random state
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Start local callback server
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	server, err := startCallbackServer(state, codeChan, errChan)
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	defer server.Shutdown(context.Background())

	// Build authorization URL
	authURL := buildAuthURL(cfg.ClientID, state, cfg.Scopes)

	// Open browser
	fmt.Printf("Opening browser to authorize...\n")
	fmt.Printf("If browser doesn't open, visit:\n%s\n\n", authURL)
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Could not open browser automatically: %v\n", err)
	}

	// Wait for callback
	fmt.Println("Waiting for authorization...")
	var code string
	select {
	case code = <-codeChan:
		// Got authorization code
	case err := <-errChan:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authorization timed out")
	}

	// Exchange code for tokens
	tokens, err := exchangeCode(cfg.ClientID, cfg.ClientSecret, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for tokens: %w", err)
	}

	return tokens, nil
}

// BuildAuthorizeURL returns the Bitbucket authorize URL plus the random state to
// match against the callback. Used by the manual (no-server) login flow.
func BuildAuthorizeURL(cfg OAuthConfig) (authURL, state string, err error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return "", "", fmt.Errorf("OAuth client credentials not configured")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = DefaultScopes()
	}
	state, err = generateState()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}
	return buildAuthURL(cfg.ClientID, state, cfg.Scopes), state, nil
}

// ExchangeCallbackURL completes the OAuth flow from a pasted callback URL (the
// full `http://localhost:34106/callback?...` redirect) or a bare authorization
// code. When the input is a full URL and expectedState is non-empty, the state
// is validated. This avoids needing the local callback server (e.g. over SSH).
func ExchangeCallbackURL(cfg OAuthConfig, expectedState, pasted string) (*TokenData, error) {
	pasted = strings.TrimSpace(pasted)
	if pasted == "" {
		return nil, fmt.Errorf("no callback URL or code provided")
	}

	code := pasted
	if u, perr := url.Parse(pasted); perr == nil && u.Query().Get("code") != "" {
		q := u.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			return nil, fmt.Errorf("authorization failed: %s - %s", errMsg, q.Get("error_description"))
		}
		if expectedState != "" && q.Get("state") != "" && q.Get("state") != expectedState {
			return nil, fmt.Errorf("state mismatch — the pasted URL is from a different login attempt")
		}
		code = q.Get("code")
	}

	tokens, err := exchangeCode(cfg.ClientID, cfg.ClientSecret, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for tokens: %w", err)
	}
	return tokens, nil
}

func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := randRead(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func buildAuthURL(clientID, state string, scopes []string) string {
	params := url.Values{
		"client_id":     {clientID},
		"response_type": {"code"},
		"redirect_uri":  {fmt.Sprintf("http://localhost:%d%s", callbackPort, callbackPath)},
		"scope":         {strings.Join(scopes, " ")},
		"state":         {state},
	}
	return authorizationURL + "?" + params.Encode()
}

func startCallbackServer(expectedState string, codeChan chan<- string, errChan chan<- error) (*http.Server, error) {
	mux := http.NewServeMux()

	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Check for error
		if errMsg := query.Get("error"); errMsg != "" {
			errDesc := query.Get("error_description")
			errChan <- fmt.Errorf("authorization failed: %s - %s", errMsg, errDesc)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Authorization Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", errDesc)
			return
		}

		// Verify state
		state := query.Get("state")
		if state != expectedState {
			errChan <- fmt.Errorf("state mismatch - possible CSRF attack")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Error</h1><p>State mismatch. Please try again.</p></body></html>")
			return
		}

		// Get authorization code
		code := query.Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Error</h1><p>No authorization code received.</p></body></html>")
			return
		}

		// Success
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><h1>Authorization Successful!</h1><p>You can close this window and return to the terminal.</p></body></html>")
		codeChan <- code
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", callbackPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", callbackPort, err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	return server, nil
}

func exchangeCode(clientID, clientSecret, code string) (*TokenData, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {fmt.Sprintf("http://localhost:%d%s", callbackPort, callbackPath)},
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClientFn().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("token exchange failed: %v", errResp)
	}

	var tokens TokenData
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}

	return &tokens, nil
}

func openBrowser(url string) error {
	return openBrowserFn(url)
}

func openBrowserReal(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}
