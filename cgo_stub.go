//go:build !cgo

// 本文件为未启用 CGO 的构建提供明确的宿主回调失败信息。

package main

import (
	"encoding/json"
	"fmt"
)

// callHost 说明宿主回调必须使用启用 CGO 的插件构建。
func callHost(method string, payload any) (json.RawMessage, error) {
	return nil, fmt.Errorf("host callback %s requires CGO", method)
}
