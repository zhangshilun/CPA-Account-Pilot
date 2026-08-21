package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	webasset "cpa-account-pilot/web"
)

type managementRequest struct {
	Method  string      `json:"Method"`
	Path    string      `json:"Path"`
	Headers http.Header `json:"Headers"`
	Query   url.Values  `json:"Query"`
	Body    []byte      `json:"Body"`
}

// handleManagement 解析 CPA 管理请求并按唯一的路由表调用处理函数。
func handleManagement(raw []byte, hostCall HostCall) ([]byte, error) {
	var request managementRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, fmt.Errorf("decode management request: %w", err)
	}
	method, path := strings.ToUpper(strings.TrimSpace(request.Method)), normalizePath(request.Path)
	switch {
	case method == http.MethodGet && path == resourceRoute:
		return OK(HTML(http.StatusOK, webasset.IndexHTML(webasset.RuntimeConfig{AccountFilesPath: accountFilesPath, AuthFilesPath: authFilesPath, VaultStatusPath: vaultStatusPath, ProviderLoginPathTemplate: providerLoginPathTemplate})))
	}
	for _, route := range managementRoutes {
		if route.Method == method && route.Path == path {
			return handleManagementRoute(method, path, request.Body, hostCall)
		}
	}
	return OK(JSON(http.StatusNotFound, map[string]string{"error": "route_not_found"}))
}

// handleManagementRoute 根据已验证的管理路由调用相应业务服务。
func handleManagementRoute(method, path string, body []byte, hostCall HostCall) ([]byte, error) {
	switch path {
	case authFilesPath:
		return ListAuthFiles(hostCall)
	case vaultStatusPath:
		return OK(JSON(http.StatusOK, VaultConfigurationStatus()))
	case accountFilesPath:
		if method == http.MethodGet {
			return ListAccountFiles()
		}
		return PersistAccountFiles(body)
	}
	return OK(JSON(http.StatusNotFound, map[string]string{"error": "route_not_found"}))
}

// normalizePath 去除请求中的查询参数并返回标准路径。
func normalizePath(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err == nil && parsed.Path != "" {
		return parsed.Path
	}
	return strings.SplitN(value, "?", 2)[0]
}
