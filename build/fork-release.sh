#!/bin/sh
# Says whether HEAD is to be released, and as which tag. The Fork release
# workflow runs it; it changes nothing, so it can be run anywhere:
#
#   build/fork-release.sh auto UPSTREAM_REF   # after a push to main
#   build/fork-release.sh dispatch            # asked for by hand
#
# The fork releases upstream version X.Y.Z (VERSION) as vX.Y.Z.1, vX.Y.Z.2
# and so on: newer than X.Y.Z and older than the next upstream version, in
# the updater's comparison, install.sh's and the Mac installer's.
#
# "auto" releases only when HEAD brought in upstream commits that the newest
# fork release does not have, so an upstream sync merged into main is
# released and fork-only changes wait for a release asked for by hand. The
# first fork release is always asked for by hand. "dispatch" releases HEAD
# unless a fork release is HEAD already.
#
# It prints release=true or false, tag= and reason= lines, also into
# $GITHUB_OUTPUT when that is set.
set -eu

fail() {
	echo "fork-release: $*" >&2
	exit 1
}

mode=${1:-}
case "$mode" in
auto) [ $# = 2 ] || fail "usage: build/fork-release.sh auto UPSTREAM_REF" ;;
dispatch) [ $# = 1 ] || fail "usage: build/fork-release.sh dispatch" ;;
*) fail "usage: build/fork-release.sh auto UPSTREAM_REF | dispatch" ;;
esac
cd "$(dirname "$0")/.."

base=$(cat VERSION)
printf '%s\n' "$base" | grep -Eqx '[0-9]+\.[0-9]+\.[0-9]+' || fail "VERSION is $base, not X.Y.Z"
head=$(git rev-parse HEAD)

# newer_than says whether version $1 is newer than version $2, as the
# updater compares them: part by part, a missing part counting as 0.
newer_than() {
	awk -v a="${1#v}" -v b="${2#v}" 'BEGIN {
		n = split(a, x, "."); m = split(b, y, ".")
		for (i = 1; i <= (n > m ? n : m); i++) {
			if (x[i] + 0 > y[i] + 0) exit 0
			if (x[i] + 0 < y[i] + 0) exit 1
		}
		exit 1
	}'
}

# The fork's releases are the four-part tags; the three-part ones are
# upstream's.
fork_tags=$(git tag -l 'v*' | grep -Ex 'v[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | sort -V || true)
latest=$(printf '%s\n' "$fork_tags" | sed '/^$/d' | tail -n 1)
n=$(printf '%s\n' "$fork_tags" | sed -n "s/^v$(printf '%s' "$base" | sed 's/\./\\./g')\.\([0-9]*\)\$/\1/p" | sort -n | tail -n 1)
tag="v$base.$((${n:-0} + 1))"
if [ -n "$latest" ] && ! newer_than "$tag" "$latest"; then
	fail "$tag would not be newer than the fork's $latest; VERSION went back?"
fi

answer() {
	echo "release=$1"
	echo "tag=$tag"
	echo "reason=$2"
	if [ -n "${GITHUB_OUTPUT:-}" ]; then
		printf 'release=%s\ntag=%s\nreason=%s\n' "$1" "$tag" "$2" >>"$GITHUB_OUTPUT"
	fi
	exit 0
}

for t in $fork_tags; do
	[ "$(git rev-parse "$t^{commit}")" != "$head" ] || answer false "HEAD is released already, as $t"
done
[ "$mode" = auto ] || answer true "asked for by hand"
[ -n "$latest" ] || answer false "no fork release yet; ask for the first one by hand (workflow_dispatch)"
upstream=$(git merge-base HEAD "$2") || fail "HEAD and $2 have nothing in common; fetch more history"
if git merge-base --is-ancestor "$upstream" "$latest"; then
	answer false "no upstream commits since $latest"
fi
answer true "upstream commits up to $(git rev-parse --short "$upstream") are new since $latest"
