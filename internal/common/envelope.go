// 本文件定义 CPA RPC 成功、错误信封及宿主响应解包逻辑。
package common

import "encoding/json"

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OKEnvelope 将成功结果封装为 CPA RPC 信封。
func OKEnvelope(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

// ErrorEnvelope 创建 CPA RPC 错误信封。
func ErrorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

// DecodeEnvelopeResult 提取 CPA 宿主回调返回的结果值。
func DecodeEnvelopeResult(raw []byte) (json.RawMessage, error) {
	var response envelope
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if !response.OK {
		if response.Error != nil {
			return nil, &callbackError{code: response.Error.Code, message: response.Error.Message}
		}
		return nil, &callbackError{code: "host_callback_failed", message: "host callback failed"}
	}
	return append(json.RawMessage(nil), response.Result...), nil
}

type callbackError struct {
	code    string
	message string
}

// Error 将宿主回调错误格式化为便于诊断的文本。
func (e *callbackError) Error() string { return e.code + ": " + e.message }
