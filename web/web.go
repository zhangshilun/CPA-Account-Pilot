// Package web embeds the plugin management page into the CPA resource route.
package web

import (
	"embed"
	"strings"
)

//go:embed index.html styles.css app.js
var assets embed.FS

// IndexHTML assembles the split source files into the single CPA resource response.
func IndexHTML() []byte {
	page, err := assets.ReadFile("index.html")
	if err != nil {
		return nil
	}
	styles, err := assets.ReadFile("styles.css")
	if err != nil {
		return nil
	}
	script, err := assets.ReadFile("app.js")
	if err != nil {
		return nil
	}
	result := strings.ReplaceAll(string(page), `<link rel="stylesheet" href="styles.css">`, "<style>\n"+string(styles)+"\n</style>")
	result = strings.ReplaceAll(result, `<script src="app.js"></script>`, "<script>\n"+string(script)+"\n</script>")
	return []byte(result)
}
