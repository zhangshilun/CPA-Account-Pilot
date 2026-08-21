package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	methodPluginRegister     = "plugin.register"
	methodPluginReconfigure  = "plugin.reconfigure"
	methodManagementRegister = "management.register"
	methodManagementHandle   = "management.handle"
	pluginABIVersion         = 1
	errorCodeInvalidMethod   = "invalid_method"
	errorCodePlugin          = "plugin_error"

	pluginID         = "cpa-account-vault"
	pluginName       = "CPA Account Vault"
	pluginAuthor     = "Solon.Z"
	pluginRepository = "https://github.com/zhangshilun/CPA-Account-Vault"
	resourcePath     = "/index.html"
	managementPrefix = "/v0/management/plugins/" + pluginID
	resourceRoute    = "/v0/resource/plugins/" + pluginID + resourcePath

	defaultVaultDirectory    = "/CLIProxyAPI/data/cpa-account-vault/"
	vaultKeyConfigName       = "CPA_ACCOUNTS_VAULT_KEY"
	vaultDirectoryConfigName = "CPA_ACCOUNTS_VAULT_DIR"
	configFieldTypeString    = "string"

	authFilesPath    = managementPrefix + "/auth-files"
	accountFilesPath = managementPrefix + "/account-files"
	vaultStatusPath  = managementPrefix + "/vault-status"

	providerLoginPathTemplate = "/v0/management/{provider}-auth-url"
)

// pluginConfig 表示 CPA 通过 config_yaml 传入的 Vault 配置。
type pluginConfig struct {
	VaultKey       string
	VaultDirectory string
}

// managementRoutes 是 CPA 注册和管理请求分发共享的唯一接口定义。
var managementRoutes = []ManagementRoute{
	{Method: http.MethodGet, Path: authFilesPath, Description: "获取脱敏后的 CPA 认证文件元数据。"},
	{Method: http.MethodGet, Path: vaultStatusPath, Description: "获取 Vault 配置状态。"},
	{Method: http.MethodPost, Path: accountFilesPath, Description: "保存加密后的账号对象文件。"},
	{Method: http.MethodGet, Path: accountFilesPath, Description: "读取账号对象并仅在内存解密密码。"},
}

// managementResources 声明插件静态管理页面资源。
var managementResources = []ResourceRoute{{Path: resourcePath, Menu: pluginName, Description: "管理私有账号文件与 CPA 凭证状态。"}}

// handleMethod 分发 CPA 生命周期、管理路由注册与管理请求。
func handleMethod(method string, request []byte, hostCall HostCall) ([]byte, error) {
	switch method {
	case methodPluginRegister:
		configureForLifecycle(request)
		return OK(pluginRegistration())
	case methodPluginReconfigure:
		configureForLifecycle(request)
		return OK(pluginRegistration())
	case methodManagementRegister:
		return OK(ManagementRegistration{Routes: managementRoutes, Resources: managementResources})
	case methodManagementHandle:
		return handleManagement(request, hostCall)
	default:
		return Error("unknown_method", "未知 CPA 插件方法: "+method), nil
	}
}

// pluginRegistration 返回 CPA 用于展示插件配置表单和能力的注册信息。
func pluginRegistration() Registration {
	return Registration{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Name: pluginName, Version: pluginVersion, Author: pluginAuthor,
			GitHubRepository: pluginRepository,
			ConfigFields: []ConfigField{
				{Name: vaultKeyConfigName, Type: configFieldTypeString, EnumValues: []string{}, Description: "必填：Base64 编码的 32 字节 AES 密钥。"},
				{Name: vaultDirectoryConfigName, Type: configFieldTypeString, EnumValues: []string{}, Description: "可选：Vault 存储目录，默认 /CLIProxyAPI/data/cpa-account-vault/。"},
			},
		},
		Capabilities: Capabilities{ManagementAPI: true},
	}
}

// configureForLifecycle 记录 Vault 配置状态；无效参数不会阻止 CPA 完成插件注册。
func configureForLifecycle(raw []byte) {
	config, err := configFromLifecycle(raw)
	if err != nil {
		SetVaultConfigurationError("Vault 配置格式无效：" + err.Error())
		return
	}
	if strings.TrimSpace(config.VaultKey) == "" {
		SetVaultConfigurationError("请在 CPA 插件配置中填写 CPA_ACCOUNTS_VAULT_KEY。")
		return
	}
	if err := ConfigureVault(config.VaultKey, config.VaultDirectory, defaultVaultDirectory); err != nil {
		SetVaultConfigurationError(err.Error())
	}
}

// configFromLifecycle 解码 CPA 生命周期请求中的 Vault 配置。
func configFromLifecycle(raw []byte) (pluginConfig, error) {
	var lifecycle LifecycleRequest
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &lifecycle); err != nil {
			return pluginConfig{}, fmt.Errorf("解析插件生命周期请求: %w", err)
		}
	}
	var config pluginConfig
	if len(strings.TrimSpace(string(lifecycle.ConfigYAML))) > 0 {
		values, err := parseVaultConfigYAML(lifecycle.ConfigYAML)
		if err != nil {
			return pluginConfig{}, fmt.Errorf("解析 Vault 插件配置: %w", err)
		}
		config.VaultKey = values[vaultKeyConfigName]
		config.VaultDirectory = values[vaultDirectoryConfigName]
	}
	return config, nil
}

// parseVaultConfigYAML 解析 CPA 配置中的顶层字符串字段，避免插件引入运行时依赖。
func parseVaultConfigYAML(raw []byte) (map[string]string, error) {
	values := make(map[string]string)
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("第 %d 行不是有效的键值配置", lineNumber+1)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if key != vaultKeyConfigName && key != vaultDirectoryConfigName {
			continue
		}
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values, nil
}
