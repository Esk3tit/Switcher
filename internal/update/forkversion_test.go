package update

import (
	"context"
	"runtime"
	"testing"
)

// The fork's releases are vX.Y.Z.N, the Nth release of upstream X.Y.Z: newer
// than X.Y.Z and every earlier fork release of it, older than the next
// upstream version and its fork releases.
func TestForkReleasesSortBetweenUpstreamVersions(t *testing.T) {
	order := []string{"v1.39.0", "1.39.0.7", "v1.40.0", "v1.40.0.1", "1.40.0.2", "v1.40.0.10", "v1.40.1", "v1.41.0", "v1.41.0.1", "v2.0.0"}
	for i, a := range order {
		for j, b := range order {
			got := compare(a, b)
			switch {
			case i < j && got >= 0, i > j && got <= 0, i == j && got != 0:
				t.Errorf("compare(%s, %s) = %d", a, b, got)
			}
		}
	}
	for _, tag := range []string{"v1.40.0", "1.40.0", "v1.40.0.1", "1.40.0.12"} {
		if !validTag.MatchString(tag) {
			t.Errorf("%s was refused", tag)
		}
	}
	for _, tag := range []string{"v1.40", "v1.40.0.1.2", "v1.40.0-fork.1", "v1.40.0+fork.1", "v1.40.0.", "v1.40.0.x", "v1.40.0.1/../x"} {
		if validTag.MatchString(tag) {
			t.Errorf("%s was accepted", tag)
		}
	}
	if tag, err := normalizeReleaseTag(" 1.40.0.3\n"); err != nil || tag != "v1.40.0.3" {
		t.Errorf("normalize: %q %v", tag, err)
	}
	for _, c := range []struct {
		current, latest string
		available       bool
	}{
		{"1.40.0", "v1.40.0.1", true},
		{"1.40.0.1", "v1.40.0.2", true},
		{"1.40.0.2", "v1.41.0.1", true},
		{"1.40.0.1", "v1.40.0.1", false},
		{"1.40.0.1", "v1.40.0", false},
	} {
		checker := New(c.current)
		checker.latest = c.latest
		if got := checker.stateLocked().Available; got != c.available {
			t.Errorf("running %s, latest %s: available %v", c.current, c.latest, got)
		}
	}
}

// A fork release installs over the upstream version it is built on: its
// asset is named, checked and smoke-tested by its four-part version.
func TestAForkReleaseInstalls(t *testing.T) {
	f := newInstallFixture(t)
	f.checker.Current, f.checker.latest, f.version = "1.40.0", "v1.40.0.1", "switcher 1.40.0.1\n"
	var asked string
	download := f.ops.download
	f.ops.download = func(ctx context.Context, tag, asset, dir string, progress func(done, total int64)) error {
		asked = tag + " " + asset
		return download(ctx, tag, asset, dir, progress)
	}
	if err := f.checker.installAndRestart(f.ops); err != nil {
		t.Fatal(err)
	}
	if want := "v1.40.0.1 switcher-server_1.40.0.1_" + runtime.GOOS + "_" + runtime.GOARCH; asked != want {
		t.Errorf("downloaded %q, want %q", asked, want)
	}
	if f.smokeCalls != 1 || f.execCalls != 1 {
		t.Errorf("smoke=%d exec=%d", f.smokeCalls, f.execCalls)
	}
	assertFixtureFile(t, f.current, "new fixture", 0o755)

	// A binary that reports upstream's version is not the fork release.
	f = newInstallFixture(t)
	f.checker.Current, f.checker.latest, f.version = "1.40.0", "v1.40.0.1", "switcher 1.40.0\n"
	if err := f.checker.installAndRestart(f.ops); err == nil || f.execCalls != 0 {
		t.Errorf("a binary of another version was installed: %v", err)
	}
}
