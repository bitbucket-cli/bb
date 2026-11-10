package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checksumFixture() string {
	return strings.Repeat("a", 64) + "  bb_windows_amd64.zip\n" +
		strings.Repeat("b", 64) + "  bb_windows_arm64.zip\n"
}

func TestManifestTargetsAndPreviewVersion(t *testing.T) {
	data, _, err := generate("v0.3.0-rc.1", "ExampleOrg/bb", strings.NewReader(checksumFixture()))
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != "0.3.0-rc.1" || got.Bin != "bb.exe" || got.License != "MIT" || got.Homepage != "https://github.com/ExampleOrg/bb" {
		t.Errorf("unexpected manifest metadata: %+v", got)
	}
	if len(got.Architecture) != 2 {
		t.Fatalf("architectures = %+v", got.Architecture)
	}
	for _, target := range targets {
		wantURL := "https://github.com/ExampleOrg/bb/releases/download/v0.3.0-rc.1/" + target.filename
		if got.Architecture[target.arch].URL != wantURL || len(got.Architecture[target.arch].Hash) != 64 {
			t.Errorf("%s download = %+v", target.arch, got.Architecture[target.arch])
		}
	}
}

func TestRejectInvalidReleaseMetadata(t *testing.T) {
	for _, tc := range []struct{ name, version, repository, checksums string }{
		{"bad version", `v1.2.3";bad`, "Example/bb", checksumFixture()},
		{"missing patch", "v1.2", "Example/bb", checksumFixture()},
		{"bad repository", "v1.2.3", `Example/bb"`, checksumFixture()},
		{"missing arm64", "v1.2.3", "Example/bb", strings.SplitN(checksumFixture(), "\n", 2)[0]},
		{"duplicate checksum", "v1.2.3", "Example/bb", checksumFixture() + strings.Split(checksumFixture(), "\n")[0] + "\n"},
		{"bad hash", "v1.2.3", "Example/bb", strings.Replace(checksumFixture(), strings.Repeat("a", 64), "zz", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := generate(tc.version, tc.repository, strings.NewReader(tc.checksums)); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func archiveBytes(t *testing.T, executable string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(executable)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("binary")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestVerifyArchivesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	var checksums strings.Builder
	for _, target := range targets {
		data := archiveBytes(t, "bb.exe")
		sum := sha256.Sum256(data)
		if err := os.WriteFile(filepath.Join(dir, target.filename), data, 0600); err != nil {
			t.Fatal(err)
		}
		checksums.WriteString(hex.EncodeToString(sum[:]) + "  " + target.filename + "\n")
	}
	checksumsPath := filepath.Join(dir, "checksums.txt")
	output := filepath.Join(dir, "bucket", "bb.json")
	if err := os.WriteFile(checksumsPath, []byte(checksums.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("v1.2.3", "Example/bb", checksumsPath, output); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bb_windows_arm64.zip"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("v2.0.0", "Example/bb", checksumsPath, output); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed generation overwrote manifest")
	}
}

func TestRejectZipWithoutRootExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb_windows_amd64.zip")
	data := archiveBytes(t, "nested/bb.exe")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if err := verifyArchive(path, hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("expected missing root bb.exe error")
	}
}
