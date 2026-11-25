package main

import (
	_ "embed"
	"strings"

	"switcher/internal/fork"
)

// upstreamVersion is upstream's VERSION: the fork's own release number is
// main.version, from FORK_VERSION.
//
//go:embed VERSION
var upstreamVersion string

func init() { fork.UpstreamVersion = strings.TrimSpace(upstreamVersion) }
