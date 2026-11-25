#!/bin/sh
# Says whether HEAD is to be released, and as which version. The Fork release
# workflow runs it; it changes nothing, so it can be run anywhere:
#
#   build/fork-release.sh auto UPSTREAM_REF    # after a push to main
#   build/fork-release.sh patch UPSTREAM_REF   # asked for by hand
#   build/fork-release.sh minor UPSTREAM_REF
#
# UPSTREAM_REF is upstream's main (00xmario/Switcher), fetched.
#
# The fork numbers its releases on its own, X.Y.Z from 1.0.0, in
# FORK_VERSION; VERSION is upstream's, and only upstream changes it. A
# release is tagged fork-vX.Y.Z: the repository also holds upstream's vX.Y.Z
# tags. A minor release (X.Y+1.0) adds or changes the fork's own features, a
# patch release (X.Y.Z+1) brings in upstream's.
#
# The version released is FORK_VERSION when no release has it yet: the first
# release, 1.0.0, or a number raised by hand ahead of the newest release.
# Otherwise it is the newest release's, raised as asked, and the workflow
# commits it to FORK_VERSION first (commit=true).
#
# "auto" releases only when HEAD brought in upstream commits that the newest
# fork release does not have, as a patch release: an upstream sync merged
# into main is released, and fork-only changes wait for a release asked for
# by hand. The first release is always asked for by hand. "patch" and
# "minor" release HEAD unless a fork release is HEAD already.
#
# It prints release=, version=, tag=, commit=, upstream_version=,
# upstream_commit= and reason= lines, also into $GITHUB_OUTPUT when that is
# set.
set -eu

fail() {
	echo "fork-release: $*" >&2
	exit 1
}

[ $# = 2 ] || fail "usage: build/fork-release.sh auto|patch|minor UPSTREAM_REF"
mode=$1
case "$mode" in
auto | patch | minor) ;;
*) fail "usage: build/fork-release.sh auto|patch|minor UPSTREAM_REF" ;;
esac
cd "$(dirname "$0")/.."

semver='(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'
current=$(cat FORK_VERSION)
printf '%s\n' "$current" | grep -Eqx "$semver" || fail "FORK_VERSION is $current, not X.Y.Z"
head=$(git rev-parse HEAD)
upstream_commit=$(git merge-base HEAD "$2") || fail "HEAD and $2 have nothing in common; fetch more history"
upstream_version=$(git show "$upstream_commit:VERSION")

# newer_than says whether version $1 is newer than version $2, as the
# updater compares them: part by part.
newer_than() {
	awk -v a="$1" -v b="$2" 'BEGIN {
		n = split(a, x, "."); m = split(b, y, ".")
		for (i = 1; i <= (n > m ? n : m); i++) {
			if (x[i] + 0 > y[i] + 0) exit 0
			if (x[i] + 0 < y[i] + 0) exit 1
		}
		exit 1
	}'
}

fork_tags=$(git tag -l 'fork-v*' | grep -Ex "fork-v$semver" | sort -V || true)
latest=$(printf '%s\n' "$fork_tags" | sed '/^$/d' | tail -n 1)
latest_version=${latest#fork-v}

version='' commit=false
answer() {
	tag=${version:+fork-v$version}
	out=$(printf 'release=%s\nversion=%s\ntag=%s\ncommit=%s\nupstream_version=%s\nupstream_commit=%s\nreason=%s\n' \
		"$1" "$version" "$tag" "$commit" "$upstream_version" "$upstream_commit" "$2")
	printf '%s\n' "$out"
	if [ -n "${GITHUB_OUTPUT:-}" ]; then
		printf '%s\n' "$out" >>"$GITHUB_OUTPUT"
	fi
	exit 0
}

for t in $fork_tags; do
	[ "$(git rev-parse "$t^{commit}")" != "$head" ] || answer false "HEAD is released already, as $t"
done
why="asked for by hand"
if [ "$mode" = auto ]; then
	[ -n "$latest" ] || answer false "no fork release yet; ask for the first one by hand (workflow_dispatch)"
	if git merge-base --is-ancestor "$upstream_commit" "$latest"; then
		answer false "no upstream commits since $latest"
	fi
	why="upstream commits up to $(git rev-parse --short "$upstream_commit") are new since $latest"
	mode='patch'
fi

if [ -z "$latest" ]; then
	version=$current
	why="$why; the fork's first release"
elif newer_than "$current" "$latest_version"; then
	version=$current
	why="$why; FORK_VERSION is $current already, ahead of $latest"
elif [ "$current" = "$latest_version" ]; then
	major=${current%%.*} rest=${current#*.}
	minor=${rest%%.*} patch=${rest#*.}
	case "$mode" in
	patch) version=$major.$minor.$((patch + 1)) ;;
	minor) version=$major.$((minor + 1)).0 ;;
	esac
	commit=true
	why="$why; a $mode release after $latest"
else
	fail "FORK_VERSION is $current, behind the fork's newest release $latest"
fi
if git rev-parse -q --verify "refs/tags/fork-v$version" >/dev/null; then
	fail "fork-v$version is taken already"
fi
answer true "$why"
