# Project guidance

## Versioning

Bump `VERSION` without being asked when committing a user-facing change (a
feature or a fix a user would notice) to main: after the change, raise it to
the next release number in its own commit, "Release vX.Y.0: <what changed>".
Bump once per release, not per commit: while `VERSION` is ahead of the newest
`v*` tag, later changes ship under that number. Build-time version strings
and the app bundle versions come from `VERSION` and the tag, so nothing else
is edited. Internal work (tests, docs, refactors), local installs and test
builds keep the current version; never invent preview versions.

Release numbers after v1.1.0 raise the middle number by one, with the last
number at 0: v1.1.0, then v1.11.0, v1.12.0, v1.13.0 and so on. Do not use
v1.2.0 or patch releases such as v1.1.1. Confirm the number with the user
right before tagging.

## Runtime and reference material

Keep active Claude Desktop and CLI sessions intact during development. Use
temporary profiles, mock credentials and isolated upstreams for verification.
Restart or reconfigure the user's running applications only when requested.

Before changing Desktop routing, native credential ownership, or setup, read
`docs/README.md` and the relevant linked contract. The pinned reference projects
and local clone paths are documented in `docs/desktop-reference-map.md`.

## This fork's releases

This is a fork of 00xmario/Switcher. Updates, install.sh and the Tailscale
add-on come from this fork's releases (`internal/fork`), signed with the
fork's key. In this fork the Versioning section above applies to
`FORK_VERSION`, not `VERSION`: `VERSION` is upstream's and is changed only by
upstream syncs.

The fork numbers its releases on its own, X.Y.Z in `FORK_VERSION` from
1.0.0, tagged `fork-vX.Y.Z` (the repository also holds upstream's `vX.Y.Z`
tags). A minor release (1.1.0, 1.2.0) adds or changes the fork's own
features; a patch release (1.0.1, 1.0.2) brings in an upstream sync. With a
user-facing fork change, raise `FORK_VERSION` to the next minor number in its
own commit, "Release fork vX.Y.0: <what changed>", once per release: while
`FORK_VERSION` is ahead of the newest `fork-v*` tag, later changes ship under
that number. Never raise it for a sync; the Fork release workflow does.

Merging an upstream sync into main (a merge, not a rebase) releases it as a
patch through the Fork release workflow (`build/fork-release.sh`), which
commits the raised `FORK_VERSION` to main and tags it. Fork-only changes ship
when that workflow is run by hand, as a patch or a minor release; confirm the
number with the user first. Every release's notes name the upstream
`VERSION` and commit it is built on. `internal/fork`'s tests fail if a sync
brings back an upstream release address or a build that takes `VERSION`.
