// 本文件提供公共的 HTML 和 JSON 管理响应构造方法。
package common

import (
	"encoding/json"
	"net/http"

	"cpa-account-pilot/internal/cpaapi"
)

// HTMLResponse 构造包含嵌入式前端 HTML 的管理响应。
func HTMLResponse(status int, body []byte) cpaapi.ManagementResponse {
	return cpaapi.ManagementResponse{StatusCode: status, Headers: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: body}
}

// JSONResponse 将值序列化为 application/json 管理响应。
func JSONResponse(status int, body any) cpaapi.ManagementResponse {
	raw, _ := json.Marshal(body)
	return cpaapi.ManagementResponse{StatusCode: status, Headers: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, Body: raw}
}
