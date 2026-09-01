// Package web embeds the server-rendered templates and the static assets, so
// the binary is self-contained and a deploy is one file plus a config.
package web

import "embed"

//go:embed all:templates
var Templates embed.FS

//go:embed all:public
var Public embed.FS
