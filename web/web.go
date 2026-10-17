// Package web embeds the static frontend assets into the server binary.
package web

import "embed"

//go:embed index.html style.css interp.js renderer.js controls.js app.js michroma-latin-400.woff2 OFL-Michroma.txt
var FS embed.FS
