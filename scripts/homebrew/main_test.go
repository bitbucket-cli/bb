package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture() string {
	var manifest strings.Builder
	for i, target := range targets {
		manifest.WriteString(strings.Repeat(string(rune('a'+i)), 64) + "  bb_" + target + ".tar.gz\n")
	}
	return manifest.String()
}

func TestFormulaTargets(t *testing.T) {
	data, hashes, err := generate("v1.2.3", "ExampleOrg/bb", strings.NewReader(fixture()))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, target := range targets {
		if !strings.Contains(text, "https://github.com/ExampleOrg/bb/releases/download/v1.2.3/bb_"+target+".tar.gz") || !strings.Contains(text, hashes[target]) {
			t.Errorf("missing URL or checksum for %s", target)
		}
	}
	for _, want := range []string{`version "1.2.3"`, "on_macos do", "on_linux do", `doc.install "README.md", "LICENSE", "THIRD_PARTY_LICENSES.txt"`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestPreviewVersion(t *testing.T) {
	data, _, err := generate("v0.3.0-rc.1", "bitbucket-cli/bb", strings.NewReader(fixture()))
	if err != nil {
		t.Fatal(err)
	}
	formula := string(data)
	if !strings.Contains(formula, `version "0.3.0-rc.1"`) || !strings.Contains(formula, "/releases/download/v0.3.0-rc.1/") {
		t.Fatal("preview tag and formula version must match")
	}
}

func TestRejectInvalidReleaseMetadata(t *testing.T) {
	cases := []struct{ name, version, repository, manifest string }{
		{"bad version", "v1.2.3\";system('bad')", "Example/bb", fixture()},
		{"missing patch", "v1.2", "Example/bb", fixture()},
		{"bad repository", "v1.2.3", "Example/bb\"", fixture()},
		{"missing target", "v1.2.3", "Example/bb", strings.SplitN(fixture(), "\n", 2)[1]},
		{"duplicate target", "v1.2.3", "Example/bb", fixture() + strings.Split(fixture(), "\n")[0] + "\n"},
		{"bad hash", "v1.2.3", "Example/bb", strings.Replace(fixture(), strings.Repeat("a", 64), "zz", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := generate(tc.version, tc.repository, strings.NewReader(tc.manifest)); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestVerifyArchivesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	var manifest strings.Builder
	for _, target := range targets {
		name := "bb_" + target + ".tar.gz"
		data := []byte(target)
		sum := sha256.Sum256(data)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		manifest.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	checksums := filepath.Join(dir, "checksums.txt")
	output := filepath.Join(dir, "Formula", "bb.rb")
	if err := os.WriteFile(checksums, []byte(manifest.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("v1.2.3", "Example/bb", checksums, output); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bb_linux_amd64.tar.gz"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("v2.0.0", "Example/bb", checksums, output); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed generation overwrote formula")
	}
}
