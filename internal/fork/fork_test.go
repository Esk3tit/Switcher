package fork

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const root = "../.."

// Nothing checks for, downloads or installs a release from upstream's
// repository: an upstream sync that brings such an address back fails here.
// Credits and links to upstream's project stay.
func TestNoReleaseComesFromUpstream(t *testing.T) {
	slug := regexp.QuoteMeta("00xmario" + "/switcher")
	upstream := regexp.MustCompile(`(?i)` + slug + `(/releases|/main/install\.sh)|repos/` + slug + `|repo=` + slug + `|= *"` + slug + `"`)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(body), "\n") {
			if upstream.MatchString(line) {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s:%d takes releases from upstream: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The scripts that cannot import Repo name the same repository.
func TestScriptsNameTheFork(t *testing.T) {
	for _, script := range []string{"install.sh", "build/release-sign.sh"} {
		body, err := os.ReadFile(filepath.Join(root, script))
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`(?m)^repo=` + regexp.QuoteMeta(Repo) + `$`).Match(body) {
			t.Errorf("%s does not set repo=%s", script, Repo)
		}
	}
	for _, file := range []string{"README.md", "docs/cli.md", "docs/linux.md", "web/app.js"} {
		body, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "https://raw.githubusercontent.com/"+Repo+"/main/install.sh") {
			t.Errorf("%s does not install from %s", file, Repo)
		}
	}
}

// Builds and releases take the fork's own number, FORK_VERSION, not
// upstream's VERSION, and tag it with TagPrefix; the Mac app names the
// fork, which tells installers its number from upstream's.
func TestBuildsUseTheForkVersion(t *testing.T) {
	read := func(file string) string {
		body, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	if v := strings.TrimSpace(read("FORK_VERSION")); !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(v) {
		t.Errorf("FORK_VERSION is %q, not X.Y.Z", v)
	}
	for file, want := range map[string]string{
		"Makefile":                           "VERSION ?= $(shell cat FORK_VERSION",
		"install.sh":                         "\ntag_prefix=" + TagPrefix + "\n",
		"build/release-sign.sh":              "version=${tag#" + TagPrefix + "}",
		".github/workflows/release.yml":      "- '" + TagPrefix + "*'",
		".github/workflows/checks.yml":       "VERSION=$(cat FORK_VERSION)",
		"build/macos/Info.plist":             "<key>SwitcherReleaseRepo</key><string>" + Repo + "</string>",
		".github/workflows/fork-release.yml": "build/fork-release.sh",
	} {
		if !strings.Contains(read(file), want) {
			t.Errorf("%s lacks %q", file, want)
		}
	}
	if got := ReleaseTag("v1.2.3"); got != TagPrefix+"1.2.3" {
		t.Errorf("ReleaseTag(v1.2.3) = %q", got)
	}
}
