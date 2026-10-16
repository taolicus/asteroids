// Package web embeds the static frontend assets into the server binary.
package web

import "embed"

//go:embed index.html style.css app.js
var FS embed.FS
