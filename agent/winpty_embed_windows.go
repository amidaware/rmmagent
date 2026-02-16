//go:build windows

package agent

import "embed"

//go:embed winpty_bins/**/*
var winptyFS embed.FS
