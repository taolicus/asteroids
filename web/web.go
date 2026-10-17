// Package web embeds the static frontend assets into the server binary.
package web

import "embed"

//go:embed index.html style.css interp.js renderer.js app.js
var FS embed.FS
