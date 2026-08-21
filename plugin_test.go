package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestPluginRegisterExposesVaultConfigFields 确保未配置密钥时 CPA 仍能读取配置表单。
func TestPluginRegisterExposesVaultConfigFields(t *testing.T) {
	raw, err := handleMethod(methodPluginRegister, nil, nil)
	if err != nil {
		t.Fatalf("注册插件: %v", err)
	}
	var response struct {
		OK     bool `json:"ok"`
		Result struct {
			Metadata struct {
				ConfigFields []ConfigField `json:"ConfigFields"`
			} `json:"metadata"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("解析注册响应: %v", err)
	}
	if !response.OK || len(response.Result.Metadata.ConfigFields) != 2 {
		t.Fatalf("配置字段未注册: %s", raw)
	}
	if response.Result.Metadata.ConfigFields[0].Name != vaultKeyConfigName || response.Result.Metadata.ConfigFields[1].Name != vaultDirectoryConfigName {
		t.Fatalf("Vault 配置字段不匹配: %#v", response.Result.Metadata.ConfigFields)
	}
}

// TestManagementRequestDecodesBinaryBody 确保 CPA JSON 信封中的正文会还原为原始 JSONL 字节。
func TestManagementRequestDecodesBinaryBody(t *testing.T) {
	input := []byte(`{"email":"user@example.com","password":"secret"}`)
	raw, err := json.Marshal(managementRequest{Method: http.MethodPost, Path: accountFilesPath, Body: input})
	if err != nil {
		t.Fatalf("编码管理请求: %v", err)
	}
	var decoded managementRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("解码管理请求: %v", err)
	}
	if string(decoded.Body) != string(input) {
		t.Fatalf("正文未还原为原始字节: got=%q want=%q", decoded.Body, input)
	}
}
