// Package docs embeds the studio-map docs page so the server binary carries it.
package docs

import _ "embed"

//go:embed index.html
var IndexHTML []byte
