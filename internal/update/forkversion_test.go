package update

import (
	"context"
	"runtime"
	"testing"

	"switcher/internal/fork"
)

// The fork numbers its releases X.Y.Z on its own, from 1.0.0, and tags them
// fork-vX.Y.Z: the fork's repository also carries upstream's vX.Y.Z tags.
func TestForkReleasesAreTaggedAndOrdered(t *testing.T) {
	order := []string{"v1.0.0", "1.0.1", "v1.0.9", "v1.0.10", "v1.1.0", "1.1.1", "v1.10.0", "v2.0.0"}
	for i, a := range order {
		for j, b := range order {
			got := compare(a, b)
			switch {
			case i < j && got >= 0, i > j && got <= 0, i == j && got != 0:
				t.Errorf("compare(%s, %s) = %d", a, b, got)
			}
		}
	}
	for tag, want := range map[string]string{"fork-v1.0.0": "v1.0.0", " fork-v1.2.10\n": "v1.2.10"} {
		if got, err := releaseVersion(tag); err != nil || got != want {
			t.Errorf("releaseVersion(%q) = %q, %v", tag, got, err)
		}
	}
	for _, tag := range []string{"v1.0.0", "1.0.0", "fork-1.0.0", "fork-vv1.0.0", "fork-v1.0", "fork-v1.40.0.1", "fork-v1.0.0-rc.1", "fork-v1.0.0/../x"} {
		if got, err := releaseVersion(tag); err == nil {
			t.Errorf("%q was taken as release %s", tag, got)
		}
	}
	if got := fork.ReleaseTag("v1.2.3"); got != "fork-v1.2.3" {
		t.Errorf("ReleaseTag(v1.2.3) = %q", got)
	}
	if got := fork.ReleaseTag("1.2.3"); got != "fork-v1.2.3" {
		t.Errorf("ReleaseTag(1.2.3) = %q", got)
	}
	for _, c := range []struct {
		current, latest string
		available       bool
	}{
		{"1.0.0", "v1.0.1", true},
		{"1.0.1", "v1.1.0", true},
		{"1.0.9", "v1.0.10", true},
		{"1.1.0", "v1.1.0", false},
		{"1.1.0", "v1.0.5", false},
	} {
		checker := New(c.current)
		checker.latest = c.latest
		if got := checker.stateLocked().Available; got != c.available {
			t.Errorf("running %s, latest %s: available %v", c.current, c.latest, got)
		}
	}
}

// A fork release installs by its own version: its asset is named, checked
// and smoke-tested as that version.
func TestAForkReleaseInstalls(t *testing.T) {
	f := newInstallFixture(t)
	f.checker.Current, f.checker.latest, f.version = "1.0.0", "v1.0.1", "switcher 1.0.1\n"
	var asked string
	download := f.ops.download
	f.ops.download = func(ctx context.Context, tag, asset, dir string, progress func(done, total int64)) error {
		asked = tag + " " + asset
		return download(ctx, tag, asset, dir, progress)
	}
	if err := f.checker.installAndRestart(f.ops); err != nil {
		t.Fatal(err)
	}
	if want := "v1.0.1 switcher-server_1.0.1_" + runtime.GOOS + "_" + runtime.GOARCH; asked != want {
		t.Errorf("downloaded %q, want %q", asked, want)
	}
	if f.smokeCalls != 1 || f.execCalls != 1 {
		t.Errorf("smoke=%d exec=%d", f.smokeCalls, f.execCalls)
	}
	assertFixtureFile(t, f.current, "new fixture", 0o755)

	// A binary that reports another version is not the release.
	f = newInstallFixture(t)
	f.checker.Current, f.checker.latest, f.version = "1.0.0", "v1.0.1", "switcher 1.40.0\n"
	if err := f.checker.installAndRestart(f.ops); err == nil || f.execCalls != 0 {
		t.Errorf("a binary of another version was installed: %v", err)
	}
}
