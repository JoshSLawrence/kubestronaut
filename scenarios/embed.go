// Package scenarios embeds the built-in scenario packs. go:embed cannot reach
// parent directories, which is why this file sits next to the packs. Adding a
// pack means adding a directory and its name to the directive below.
package scenarios

import "embed"

// FS holds one top-level directory per pack.
//
//go:embed cka
var FS embed.FS
