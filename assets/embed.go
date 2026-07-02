package assets

import "embed"

// FS contains the built-in Amazonas MVP content.
//
//go:embed *.json
var FS embed.FS
