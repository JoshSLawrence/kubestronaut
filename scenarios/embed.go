// Package scenarios embeds the built-in scenario packs. go:embed cannot reach
// parent directories, which is why this file sits next to the packs. Every
// top-level directory is embedded, so adding a pack means adding a directory
// and nothing else. Stray top-level files (this one, dotfiles) are harmless
// because the loader only reads directories that contain a pack.yaml.
package scenarios

import "embed"

// FS holds one top-level directory per pack.
//
//go:embed *
var FS embed.FS
