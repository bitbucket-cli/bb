// Package git provides small helpers for interacting with the local git
// repository — detecting the Bitbucket workspace/repo from the origin remote,
// reading the current branch, and running git commands.
package git

import (
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// ValidateCloneURL accepts only Bitbucket Cloud's HTTPS and SSH clone URLs.
// Reject Git options, local paths and remote-helper protocols before invoking Git.
func ValidateCloneURL(raw string) error {
	invalid := fmt.Errorf("clone URL must be a Bitbucket Cloud HTTPS or SSH URL")
	if strings.ContainsAny(raw, "\r\n\x00") {
		return invalid
	}
	path := ""
	if strings.HasPrefix(raw, "git@bitbucket.org:") {
		path = strings.TrimPrefix(raw, "git@bitbucket.org:")
	} else {
		u, err := url.Parse(raw)
		if err != nil || u.RawQuery != "" || u.Fragment != "" {
			return invalid
		}
		switch u.Scheme {
		case "https":
			if u.Hostname() != "bitbucket.org" || (u.Port() != "" && u.Port() != "443") {
				return invalid
			}
		case "ssh":
			if u.Hostname() != "bitbucket.org" || (u.Port() != "" && u.Port() != "22") || u.User == nil || u.User.Username() != "git" {
				return invalid
			}
		default:
			return invalid
		}
		if u.User != nil {
			if _, hasPassword := u.User.Password(); hasPassword {
				return invalid
			}
		}
		path = strings.TrimPrefix(u.Path, "/")
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return invalid
	}
	return nil
}

// runGit is overridable in tests.
var runGit = func(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// bitbucketRemote matches the workspace/repo in an SSH or HTTPS Bitbucket
// remote URL, with an optional userinfo prefix and optional .git suffix.
var bitbucketRemote = regexp.MustCompile(`bitbucket\.org[:/]([^/]+)/(.+?)(?:\.git)?/?$`)

// ParseRemoteURL extracts the workspace and repo slug from a Bitbucket remote
// URL (SSH, HTTPS, or ssh:// form).
func ParseRemoteURL(remote string) (workspace, repoSlug string, err error) {
	m := bitbucketRemote.FindStringSubmatch(strings.TrimSpace(remote))
	if m == nil {
		return "", "", fmt.Errorf("not a recognised Bitbucket remote URL: %q", remote)
	}
	return m[1], m[2], nil
}

// DetectRepo returns the Bitbucket workspace and repo slug for the current
// directory by inspecting the origin remote.
func DetectRepo() (workspace, repoSlug string, err error) {
	remote, err := runGit("remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("could not read origin remote (are you in a git repository?): %w", err)
	}
	return ParseRemoteURL(remote)
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch() (string, error) {
	return runGit("rev-parse", "--abbrev-ref", "HEAD")
}

// Fetch fetches a ref from the given remote.
func Fetch(remote, ref string) error {
	_, err := runGit("fetch", remote, ref)
	return err
}

// Checkout checks out the given branch.
func Checkout(branch string) error {
	_, err := runGit("checkout", branch)
	return err
}
