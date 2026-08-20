// Package cpaapi defines the minimal CPA protocol surface used by this plugin.
package cpaapi

import (
	"net/http"
	"time"
)

const (
	SchemaVersion = 1

	MethodPluginRegister     = "plugin.register"
	MethodPluginReconfigure  = "plugin.reconfigure"
	MethodManagementRegister = "management.register"
	MethodManagementHandle   = "management.handle"
	MethodHostAuthList       = "host.auth.list"
)

type Metadata struct {
	Name             string
	Version          string
	Author           string
	GitHubRepository string
	Logo             string
	ConfigFields     []ConfigField
}

type ConfigField struct {
	Name        string
	Type        string
	Description string
}

type Registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      Metadata                 `json:"metadata"`
	Capabilities  RegistrationCapabilities `json:"capabilities"`
}

type RegistrationCapabilities struct {
	ManagementAPI bool `json:"management_api"`
	UsagePlugin   bool `json:"usage_plugin"`
}

type ManagementRegistrationResponse struct {
	Routes    []ManagementRoute `json:"routes,omitempty"`
	Resources []ResourceRoute   `json:"resources,omitempty"`
}

type ManagementRoute struct {
	Method      string
	Path        string
	Menu        string
	Description string
}

type ResourceRoute struct {
	Path        string
	Menu        string
	Description string
}

type ManagementResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type HostAuthFileEntry struct {
	ID            string    `json:"id,omitempty"`
	AuthIndex     string    `json:"auth_index,omitempty"`
	Email         string    `json:"email,omitempty"`
	Account       string    `json:"account,omitempty"`
	Status        string    `json:"status,omitempty"`
	StatusMessage string    `json:"status_message,omitempty"`
	Disabled      bool      `json:"disabled,omitempty"`
	Unavailable   bool      `json:"unavailable,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type HostAuthListResponse struct {
	Files []HostAuthFileEntry `json:"files"`
}
