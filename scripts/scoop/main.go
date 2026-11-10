// Command scoop generates a manifest from verified GoReleaser Windows archives.
// It writes a local file only; publishing belongs to the release workflow.
package main

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
var repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)

var targets = []struct {
	arch, filename string
}{
	{"64bit", "bb_windows_amd64.zip"},
	{"arm64", "bb_windows_arm64.zip"},
}

type download struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

type manifest struct {
	Version      string              `json:"version"`
	Description  string              `json:"description"`
	Homepage     string              `json:"homepage"`
	License      string              `json:"license"`
	Architecture map[string]download `json:"architecture"`
	Bin          string              `json:"bin"`
}

func main() {
	version := flag.String("version", "", "release tag, e.g. v1.2.3")
	repository := flag.String("repository", "bitbucket-cli/bb", "GitHub owner/repository containing the release")
	checksums := flag.String("checksums", "dist/checksums.txt", "GoReleaser checksums file; archives must be alongside it")
	output := flag.String("output", "dist/scoop/bb.json", "local manifest output path")
	flag.Parse()
	if err := run(*version, *repository, *checksums, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(version, repository, checksumsPath, output string) error {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}
	result, hashes, err := generate(version, repository, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	for _, target := range targets {
		path := filepath.Join(filepath.Dir(checksumsPath), target.filename)
		if err := verifyArchive(path, hashes[target.filename]); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	return os.WriteFile(output, result, 0644)
}

func generate(version, repository string, checksums io.Reader) ([]byte, map[string]string, error) {
	if !versionPattern.MatchString(version) {
		return nil, nil, fmt.Errorf("invalid release tag %q; expected vMAJOR.MINOR.PATCH with optional prerelease", version)
	}
	if !repositoryPattern.MatchString(repository) {
		return nil, nil, fmt.Errorf("invalid GitHub repository %q; expected owner/repository", repository)
	}
	hashes := make(map[string]string)
	scanner := bufio.NewScanner(checksums)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, nil, fmt.Errorf("invalid checksum line")
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil || len(sum) != sha256.Size {
			return nil, nil, fmt.Errorf("invalid SHA256 checksum for %s", fields[1])
		}
		name := strings.TrimPrefix(fields[1], "*")
		if _, exists := hashes[name]; exists {
			return nil, nil, fmt.Errorf("duplicate checksum entry for %s", name)
		}
		hashes[name] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	architecture := make(map[string]download, len(targets))
	for _, target := range targets {
		hash := hashes[target.filename]
		if hash == "" {
			return nil, nil, fmt.Errorf("missing checksum for %s", target.filename)
		}
		architecture[target.arch] = download{
			URL:  fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repository, version, target.filename),
			Hash: hash,
		}
	}
	result, err := json.MarshalIndent(manifest{
		Version:      strings.TrimPrefix(version, "v"),
		Description:  "Bitbucket CLI for repositories, pull requests, and pipelines",
		Homepage:     "https://github.com/" + repository,
		License:      "MIT",
		Architecture: architecture,
		Bin:          "bb.exe",
	}, "", "    ")
	return append(result, '\n'), hashes, err
}

func verifyArchive(path, wantHash string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != wantHash {
		return fmt.Errorf("archive checksum mismatch: %s", filepath.Base(path))
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name == "bb.exe" && !entry.FileInfo().IsDir() {
			return nil
		}
	}
	return fmt.Errorf("archive has no root bb.exe: %s", filepath.Base(path))
}
