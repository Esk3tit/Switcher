// Package fork names where this fork of Switcher publishes its releases and
// the keys they are signed with. Everything that checks for, downloads or
// verifies a release reads it from here, so an upstream sync conflicts in
// this one file, if anywhere.
//
// install.sh and build/release-sign.sh cannot import Go; fork_test.go and
// internal/update's tests keep their copies equal to these.
package fork

// Repo is the GitHub slug the fork's releases live in.
const Repo = "Esk3tit/Switcher"

// ReleaseKeys are the Ed25519 public keys, as the second field of their .pub
// files, that a release's SHA256SUMS may be signed with (ssh-keygen -Y sign
// -n switcher-release): the key the Release workflow signs with, kept as the
// SWITCHER_RELEASE_SIGNING_KEY Actions secret, and one kept offline should
// that one be lost. build/fork-release-key.sh makes both and writes them here
// and into install.sh. Until it has, no release verifies and the Release
// workflow refuses to publish one.
var ReleaseKeys = []string{
	"AAAAC3NzaC1lZDI1NTE5AAAAICJzT3kkppYEpDMzEvTM1VFj5DZvvn4ZCNoc77Z+pEc3", // switcher-fork-release-1, the Release workflow signs with it
	"AAAAC3NzaC1lZDI1NTE5AAAAIFg+mUrEJyJTPdzjMI39Glx3M2x0DVDWldfyaDMDNyop", // switcher-fork-release-2, offline
}
