// Package fork names where this fork of Switcher publishes its releases, how
// they are tagged and the keys they are signed with. Everything that checks
// for, downloads or verifies a release reads it from here, so an upstream
// sync conflicts in this one file, if anywhere.
//
// The fork numbers its releases on its own, in FORK_VERSION from 1.0.0; the
// VERSION file stays upstream's. install.sh, build/release-sign.sh and the
// workflows cannot import Go; fork_test.go and internal/update's tests keep
// their copies equal to these.
package fork

import "strings"

// Repo is the GitHub slug the fork's releases live in.
const Repo = "Esk3tit/Switcher"

// TagPrefix starts every fork release tag: fork-v1.0.0 is release 1.0.0.
// The fork's repository carries upstream's own vX.Y.Z tags, which the fork's
// numbers would otherwise collide with.
const TagPrefix = "fork-v"

// ReleaseTag is the tag of release version, given as X.Y.Z or vX.Y.Z.
func ReleaseTag(version string) string {
	return TagPrefix + strings.TrimPrefix(version, "v")
}

// UpstreamVersion is the upstream release this build is based on (upstream's
// VERSION), set by package main. A paired computer reads it to tell which
// upstream features a fork host has: the fork's own numbers say nothing
// about that. Empty in tests.
var UpstreamVersion string

// ReleaseKeys are the Ed25519 public keys, as the second field of their .pub
// files, that a release's SHA256SUMS may be signed with (ssh-keygen -Y sign
// -n switcher-release): the key the Release workflow signs with, kept as the
// SWITCHER_RELEASE_SIGNING_KEY Actions secret, and one kept offline should
// that one be lost. build/fork-release-key.sh makes both and writes them here
// and into install.sh. Until it has, no release verifies and the Release
// workflow refuses to publish one.
var ReleaseKeys = []string{}
