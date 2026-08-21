// Package web embeds the plugin management page into the CPA resource route.
package web

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed index.html styles.css app.js
var assets embed.FS

// RuntimeConfig 是管理页面从后端读取的运行时接口配置。
type RuntimeConfig struct {
	AccountFilesPath          string            `json:"accountFilesPath"`
	AuthFilesPath             string            `json:"authFilesPath"`
	VaultStatusPath           string            `json:"vaultStatusPath"`
	ProviderLoginPathTemplate string            `json:"provider_login_path_template"`
}

// IndexHTML 将静态资源合并为单个响应，并写入后端提供的管理接口配置。
func IndexHTML(config RuntimeConfig) []byte {
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
	runtimeConfig, err := json.Marshal(config)
	if err != nil {
		return nil
	}
	result := strings.ReplaceAll(string(page), `<link rel="stylesheet" href="styles.css">`, "<style>\n"+string(styles)+"\n</style>")
	result = strings.ReplaceAll(result, `<script src="app.js"></script>`, "<script>\n"+string(script)+"\n</script>")
	result = strings.ReplaceAll(result, `__CPA_PLUGIN_RUNTIME_CONFIG__`, string(runtimeConfig))
	return []byte(result)
}
