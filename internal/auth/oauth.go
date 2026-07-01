package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
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

// DefaultScopes returns the default OAuth scopes for the CLI
func DefaultScopes() []string {
	return []string{
		"repository",
		"repository:write",
		"repository:admin",
		"pullrequest",
		"pullrequest:write",
		"pipeline",
		"pipeline:write",
		"account",
	}
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
			fmt.Fprintf(w, "<html><body><h1>Authorization Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", html.EscapeString(errDesc))
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
