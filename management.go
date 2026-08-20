// 本文件解析 CPA 管理请求并将路由交给相应功能服务。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cpa-account-pilot/internal/common"
	"cpa-account-pilot/internal/service"
	webasset "cpa-account-pilot/web"
)

type managementRequest struct {
	Method         string
	Path           string
	Headers        http.Header
	Query          url.Values
	Body           []byte
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

// handleManagement 解析管理请求并分发到对应路由。
func handleManagement(raw []byte, hostCall common.HostCall) ([]byte, error) {
	var req managementRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("decode management request: %w", err)
		}
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	path := normalizePath(req.Path)
	switch {
	case method == http.MethodGet && path == resourceRoute:
		return common.OKEnvelope(common.HTMLResponse(http.StatusOK, webasset.IndexHTML()))
	case method == http.MethodGet && path == accountsRoute:
		return service.ListPrivateAccounts(hostCall)
	case method == http.MethodPost && path == accountsImportRoute:
		return service.ImportPrivateAccounts(req.Body)
	case method == http.MethodPut && path == accountRoute:
		return service.UpsertPrivateAccount(req.Body)
	case method == http.MethodDelete && path == accountRoute:
		return service.DeletePrivateAccount(req.Body)
	case method == http.MethodGet && path == authFilesRoute:
		return service.ListAuthFiles(hostCall)
	case method == http.MethodPost && path == accountFilesRoute:
		return service.PersistAccountFiles(req.Body)
	case method == http.MethodGet && path == accountFilesRoute:
		return service.ListAccountFiles()
	default:
		return common.OKEnvelope(common.JSONResponse(http.StatusNotFound, map[string]string{"error": "route_not_found"}))
	}
}

// normalizePath 移除查询参数并规范化传入的 URL 路径。
func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if parsed, err := url.Parse(path); err == nil && parsed.Path != "" {
		return parsed.Path
	}
	return strings.SplitN(path, "?", 2)[0]
}
