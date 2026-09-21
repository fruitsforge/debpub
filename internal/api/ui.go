package api

import (
	_ "embed"
)

// IndexHTML contains the embedded single-page HTML application for the Debian repository browser.
//
//go:embed ui/index.html
var IndexHTML string
