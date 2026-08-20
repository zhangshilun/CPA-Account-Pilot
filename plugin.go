// 本文件处理 CPA 插件注册、能力声明和请求分发。
package main

import (
	"net/http"

	"cpa-account-pilot/internal/common"
	"cpa-account-pilot/internal/cpaapi"
	"cpa-account-pilot/internal/service"
)

const (
	pluginID            = "cpa-account-pilot"
	pluginName          = "CPA Account Pilot"
	pluginRepository    = "https://github.com/zhangshilun/CPA-Account-Pilot"
	resourcePath        = "/index.html"
	resourceRoute       = "/v0/resource/plugins/" + pluginID + resourcePath
	managementPrefix    = "/v0/management/plugins/" + pluginID
	accountsRoute       = managementPrefix + "/accounts"
	accountsImportRoute = managementPrefix + "/accounts/import"
	accountRoute        = managementPrefix + "/account"
	authFilesRoute      = managementPrefix + "/auth-files"
	accountFilesRoute   = managementPrefix + "/account-files"
)

// handleMethod 分发 CPA 插件生命周期和管理调用。
func handleMethod(method string, request []byte, version string, hostCall common.HostCall) ([]byte, error) {
	switch method {
	case cpaapi.MethodPluginRegister, cpaapi.MethodPluginReconfigure:
		if err := service.InitializePrivateAccountStore(); err != nil {
			return nil, err
		}
		return common.OKEnvelope(cpaapi.Registration{
			SchemaVersion: cpaapi.SchemaVersion,
			Metadata: cpaapi.Metadata{
				Name: pluginName, Version: version, Author: "CPA Account Pilot",
				GitHubRepository: pluginRepository,
				ConfigFields:     []cpaapi.ConfigField{},
			},
			Capabilities: cpaapi.RegistrationCapabilities{ManagementAPI: true},
		})
	case cpaapi.MethodManagementRegister:
		return common.OKEnvelope(cpaapi.ManagementRegistrationResponse{
			Routes: []cpaapi.ManagementRoute{
				{Method: http.MethodGet, Path: accountsRoute, Description: "List private accounts and credential link states."},
				{Method: http.MethodPost, Path: accountsImportRoute, Description: "Import private accounts."},
				{Method: http.MethodPut, Path: accountRoute, Description: "Create or update a private account."},
				{Method: http.MethodDelete, Path: accountRoute, Description: "Delete a private account record."},
				{Method: http.MethodGet, Path: authFilesRoute, Description: "List CPA authentication-file metadata for account association."},
				{Method: http.MethodPost, Path: accountFilesRoute, Description: "Persist imported account objects as individual JSON files."},
				{Method: http.MethodGet, Path: accountFilesRoute, Description: "List persisted account objects."},
			},
			Resources: []cpaapi.ResourceRoute{{Path: resourcePath, Menu: pluginName, Description: "View the CLIProxyAPI authentication file list."}},
		})
	case cpaapi.MethodManagementHandle:
		return handleManagement(request, hostCall)
	default:
		return common.ErrorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}
