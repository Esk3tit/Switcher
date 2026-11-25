#!/bin/sh
# Makes the fork's release keys, once, on your own computer:
#
#   build/fork-release-key.sh            # then commit and push what it changed
#   build/fork-release-key.sh --no-secret
#
# - two Ed25519 keys in ~/.config/switcher-release (SWITCHER_RELEASE_DIR names
#   another folder): fork-1, which the Release workflow signs with, and
#   fork-2, to keep offline and sign with should fork-1 be lost;
# - fork-1 becomes the SWITCHER_RELEASE_SIGNING_KEY Actions secret of the fork
#   (with gh; --no-secret leaves that to you);
# - both public keys go into internal/fork/fork.go and install.sh, which is
#   what Switcher and the installer trust.
#
# It never replaces keys that are there already: a key Switcher trusts is
# changed only by a release that trusts the new one too.
set -eu

fail() {
	echo "fork-release-key: $*" >&2
	exit 1
}

secret=1
case "${1:-}" in
"") ;;
--no-secret) secret= ;;
*) fail "usage: build/fork-release-key.sh [--no-secret]" ;;
esac
cd "$(dirname "$0")/.."
repo=$(sed -n 's/^const Repo = "\(.*\)"$/\1/p' internal/fork/fork.go)
[ -n "$repo" ] || fail "internal/fork/fork.go names no Repo"
grep -qx 'var ReleaseKeys = \[\]string{}' internal/fork/fork.go ||
	fail "internal/fork/fork.go has release keys already"
grep -qx "release_signers=''" install.sh || fail "install.sh has release keys already"
command -v ssh-keygen >/dev/null 2>&1 || fail "ssh-keygen is needed"
if [ -n "$secret" ]; then
	command -v gh >/dev/null 2>&1 || fail "gh is needed to set the secret (or run with --no-secret)"
	gh auth status >/dev/null 2>&1 || fail "gh is not signed in (gh auth login)"
fi

dir=${SWITCHER_RELEASE_DIR:-$HOME/.config/switcher-release}
mkdir -p "$dir"
chmod 0700 "$dir"
for n in 1 2; do
	[ ! -e "$dir/fork-$n" ] || fail "$dir/fork-$n exists already"
done
for n in 1 2; do
	ssh-keygen -q -t ed25519 -N "" -C "switcher-fork-release-$n" -f "$dir/fork-$n"
done
key1=$(cut -d' ' -f2 "$dir/fork-1.pub")
key2=$(cut -d' ' -f2 "$dir/fork-2.pub")

# A signature by fork-1 checks out with the keys as install.sh will list them.
# shellcheck disable=SC2089,SC2090 # the quotes are part of allowed_signers
signers="switcher-release namespaces=\"switcher-release\" ssh-ed25519 $key1 switcher-fork-release-1
switcher-release namespaces=\"switcher-release\" ssh-ed25519 $key2 switcher-fork-release-2"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
printf '%s\n' "$signers" >"$work/allowed_signers"
echo test >"$work/SHA256SUMS"
ssh-keygen -q -Y sign -f "$dir/fork-1" -n switcher-release "$work/SHA256SUMS" </dev/null
ssh-keygen -Y verify -f "$work/allowed_signers" -I switcher-release -n switcher-release \
	-s "$work/SHA256SUMS.sig" <"$work/SHA256SUMS" >/dev/null || fail "the new key does not verify"

# shellcheck disable=SC2090
export key1 key2 signers
awk '
	$0 == "var ReleaseKeys = []string{}" {
		print "var ReleaseKeys = []string{"
		print "\t\"" ENVIRON["key1"] "\", // switcher-fork-release-1, the Release workflow signs with it"
		print "\t\"" ENVIRON["key2"] "\", // switcher-fork-release-2, offline"
		print "}"
		next
	}
	{ print }' internal/fork/fork.go >"$work/fork.go"
awk '
	$0 == "release_signers=\047\047" { print "release_signers=\047" ENVIRON["signers"] "\047"; next }
	{ print }' install.sh >"$work/install.sh"
cat "$work/fork.go" >internal/fork/fork.go
cat "$work/install.sh" >install.sh

if [ -n "$secret" ]; then
	gh secret set SWITCHER_RELEASE_SIGNING_KEY --repo "$repo" <"$dir/fork-1"
	echo "Set SWITCHER_RELEASE_SIGNING_KEY on $repo."
fi
echo "Made $dir/fork-1 (the Release workflow's) and $dir/fork-2."
echo "Keep fork-2 offline, away from GitHub: it signs (build/release-sign.sh) should fork-1 be lost."
echo "Commit internal/fork/fork.go and install.sh and push them to main."
