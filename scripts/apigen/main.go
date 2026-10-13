// Command apigen downloads a pinned schema and regenerates the Bitbucket client.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed source.json
var sourceJSON []byte

//go:embed compatibility.json
var compatibilityJSON []byte

type source struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Generator string `json:"generator"`
}

type operation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

func main() {
	input := flag.String("schema", "", "use an already-downloaded schema (the pinned checksum is still required)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := generate(ctx, *input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(ctx context.Context, input string) error {
	var pin source
	if err := json.Unmarshal(sourceJSON, &pin); err != nil {
		return err
	}
	var data []byte
	var err error
	if input == "" {
		data, err = download(ctx, pin.URL, &http.Client{Timeout: time.Minute})
	} else {
		data, err = os.ReadFile(input)
	}
	if err != nil {
		return err
	}
	prepared, err := prepare(data, pin.SHA256, compatibilityJSON)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "bb-apigen-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	schema := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(schema, prepared, 0600); err != nil {
		return err
	}
	generated := filepath.Join(tmp, "client.go")
	cmd := exec.CommandContext(ctx, "go", "run", pin.Generator, "-generate", "types,client", "-package", "bitbucket", "-o", generated, schema)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate client: %w", err)
	}
	content, err := os.ReadFile(generated)
	if err != nil {
		return err
	}
	output := filepath.Join("generated", "bitbucket", "client.go")
	// Stage beside the destination, so generation/download failures leave it intact.
	stage, err := os.CreateTemp(filepath.Dir(output), ".client-*.go")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	if _, err := stage.Write(content); err != nil {
		_ = stage.Close()
		return err
	}
	if err := stage.Chmod(0644); err != nil {
		_ = stage.Close()
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	return os.Rename(stage.Name(), output)
}

func download(ctx context.Context, location string, client *http.Client) ([]byte, error) {
	u, err := url.Parse(location)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("schema URL must use HTTPS without embedded credentials")
	}
	transport := *client
	transport.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.User != nil {
			return fmt.Errorf("refusing insecure or excessive schema redirects")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	resp, err := transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("schema download: HTTP %d", resp.StatusCode)
	}
	const limit = 32 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("schema exceeds 32 MiB limit")
	}
	return data, nil
}

func prepare(data []byte, expected string, patch []byte) ([]byte, error) {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return nil, fmt.Errorf("schema checksum mismatch: got %x; review upstream changes before updating source.json and compatibility.json", sum)
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if doc["openapi"] != "3.0.0" {
		return nil, fmt.Errorf("expected OpenAPI 3.0.0")
	}
	strip(doc, false)
	if err := applyPatch(doc, patch); err != nil {
		return nil, err
	}
	// Strip again to prevent compatibility additions from introducing upstream prose.
	strip(doc, false)
	return json.MarshalIndent(doc, "", "  ")
}

// Named maps contain API names, not OpenAPI keyword fields. Literal data such as
// enums/defaults is opaque: a field named "description" there is actual API data.
func strip(value any, namedMap bool) {
	switch x := value.(type) {
	case []any:
		for _, v := range x {
			strip(v, false)
		}
	case map[string]any:
		for k, v := range x {
			if namedMap {
				strip(v, false)
				continue
			}
			switch k {
			case "description", "summary", "example", "examples", "externalDocs", "contact", "license", "termsOfService", "x-atlassian-narrative":
				delete(x, k)
			case "enum", "default", "const", "discriminator", "security", "mapping", "required":
				// Preserve values affecting the API contract.
			case "paths", "properties", "schemas", "responses", "headers", "securitySchemes", "definitions", "patternProperties", "links", "callbacks", "$defs":
				strip(v, true)
			default:
				strip(v, false)
			}
		}
	}
}

// Compatibility uses JSON Patch add/remove/replace operations on object keys.
// Arrays are replaced as a whole; other patch operations are deliberately rejected.
func applyPatch(doc map[string]any, data []byte) error {
	var ops []operation
	if err := json.Unmarshal(data, &ops); err != nil {
		return err
	}
	for _, op := range ops {
		if !strings.HasPrefix(op.Path, "/") {
			return fmt.Errorf("invalid compatibility path %q", op.Path)
		}
		parts := strings.Split(op.Path[1:], "/")
		for i, part := range parts {
			parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		}
		parent := doc
		for _, key := range parts[:len(parts)-1] {
			next, ok := parent[key].(map[string]any)
			if !ok {
				return fmt.Errorf("compatibility parent missing at %q", op.Path)
			}
			parent = next
		}
		key := parts[len(parts)-1]
		_, exists := parent[key]
		switch op.Op {
		case "remove", "replace":
			if !exists {
				return fmt.Errorf("compatibility target missing at %q", op.Path)
			}
		case "add":
		default:
			return fmt.Errorf("unsupported compatibility operation %q", op.Op)
		}
		if op.Op == "remove" {
			delete(parent, key)
		} else {
			var v any
			dec := json.NewDecoder(bytes.NewReader(op.Value))
			dec.UseNumber()
			if err := dec.Decode(&v); err != nil {
				return err
			}
			parent[key] = v
		}
	}
	return nil
}
