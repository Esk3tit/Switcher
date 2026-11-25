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
