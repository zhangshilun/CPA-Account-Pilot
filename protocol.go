// 本文件提供插件协议和管理响应的内部实现。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SchemaVersion 表示 CPA 插件 RPC 协议版本。
const SchemaVersion uint32 = 2

// HostCall 表示经由 CGO 桥接发起的 CPA 宿主回调。
type HostCall func(method string, payload any) (json.RawMessage, error)

// Envelope 表示 CPA RPC 的统一成功或失败信封。
type Envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

// RPCError 表示 CPA RPC 的结构化错误。
type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK 将值编码为成功 RPC 信封。
func OK(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{OK: true, Result: raw})
}

// Error 将错误编码为失败 RPC 信封。
func Error(code, message string) []byte {
	raw, _ := json.Marshal(Envelope{OK: false, Error: &RPCError{Code: code, Message: message}})
	return raw
}

// DecodeResult 校验宿主回调信封并提取结果。
func DecodeResult(raw []byte) (json.RawMessage, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("解码宿主回调响应: %w", err)
	}
	if envelope.OK {
		return append(json.RawMessage(nil), envelope.Result...), nil
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return nil, fmt.Errorf("宿主回调失败")
}

// JSON 创建 JSON 格式的 CPA 管理响应。
func JSON(status int, body any) ManagementResponse {
	raw, err := json.Marshal(body)
	if err != nil {
		raw, status = []byte(`{"error":"response_encoding_failed"}`), http.StatusInternalServerError
	}
	return ManagementResponse{StatusCode: status, Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: raw}
}

// HTML 创建 HTML 格式的 CPA 管理响应。
func HTML(status int, body []byte) ManagementResponse {
	return ManagementResponse{StatusCode: status, Headers: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: body}
}

// LifecycleRequest 表示插件注册或重配请求。
type LifecycleRequest struct {
	SchemaVersion uint32 `json:"schema_version"`
	ConfigYAML    []byte `json:"config_yaml"`
}

// Metadata 表示 CPA 管理界面使用的插件元数据。
type Metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	Logo             string        `json:"Logo"`
	ConfigFields     []ConfigField `json:"ConfigFields"`
}

// ConfigField 描述一个 CPA 插件配置项。
type ConfigField struct {
	Name        string   `json:"Name"`
	Type        string   `json:"Type"`
	EnumValues  []string `json:"EnumValues"`
	Description string   `json:"Description"`
}

// Registration 表示插件注册响应。
type Registration struct {
	SchemaVersion uint32       `json:"schema_version"`
	Metadata      Metadata     `json:"metadata"`
	Capabilities  Capabilities `json:"capabilities"`
}

// Capabilities 表示插件启用的 CPA 能力。
type Capabilities struct {
	ManagementAPI bool `json:"management_api"`
}

// ManagementRegistration 表示管理路由和资源注册响应。
type ManagementRegistration struct {
	Routes    []ManagementRoute `json:"routes,omitempty"`
	Resources []ResourceRoute   `json:"resources,omitempty"`
}

// ManagementRoute 描述一个受 CPA 管理密钥保护的接口。
type ManagementRoute struct {
	Method      string `json:"Method"`
	Path        string `json:"Path"`
	Description string `json:"Description,omitempty"`
}

// ResourceRoute 描述一个静态资源入口。
type ResourceRoute struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

// ManagementResponse 表示插件交给 CPA 的 HTTP 响应。
type ManagementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers,omitempty"`
	Body       []byte      `json:"Body,omitempty"`
}

// HostAuthListResponse 表示 host.auth.list 的返回内容。
type HostAuthListResponse struct {
	Files []HostAuthFile `json:"files"`
}

// HostAuthFile 表示可安全公开的 CPA 认证文件元数据。
type HostAuthFile struct {
	ID            string `json:"id,omitempty"`
	AuthIndex     string `json:"auth_index,omitempty"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Status        string `json:"status,omitempty"`
	StatusMessage string `json:"status_message,omitempty"`
	Disabled      bool   `json:"disabled,omitempty"`
	Unavailable   bool   `json:"unavailable,omitempty"`
	Websockets    bool   `json:"websockets,omitempty"`
}

// HostAuthGetRequest 表示 host.auth.get 的请求内容。
type HostAuthGetRequest struct {
	AuthIndex string `json:"auth_index"`
}

// HostAuthGetResponse 表示 host.auth.get 的响应；JSON 仅在服务端提取允许字段。
type HostAuthGetResponse struct {
	AuthIndex string          `json:"auth_index,omitempty"`
	Name      string          `json:"name,omitempty"`
	Email     string          `json:"email,omitempty"`
	PlanType  string          `json:"plan_type,omitempty"`
	Disabled  bool            `json:"disabled,omitempty"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
	JSON      json.RawMessage `json:"json,omitempty"`
}
