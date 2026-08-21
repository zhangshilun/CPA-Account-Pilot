//go:build !cgo

package main

import (
	"encoding/json"
	"fmt"
)

// callHost 在未启用 CGO 的构建中返回明确的宿主回调不可用错误。
func callHost(method string, payload any) (json.RawMessage, error) {
	return nil, fmt.Errorf("host callback %s requires CGO", method)
}
