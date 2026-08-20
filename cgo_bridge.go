//go:build cgo

// 本文件负责将 CPA 的 C ABI 调用适配到 Go 服务层。

package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct { void* ptr; size_t len; } cliproxy_buffer;
typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);
typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;
typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);
typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static const cliproxy_host_api* stored_host;
static void store_host_api(const cliproxy_host_api* host) { stored_host = host; }
static void clear_host_api(void) { stored_host = NULL; }
static int call_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) return 1;
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}
static void free_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) stored_host->free_buffer(ptr, len);
}
static int write_plugin_response(const uint8_t* raw, size_t raw_len, cliproxy_buffer* response) {
	if (response == NULL || raw == NULL || raw_len == 0) return 0;
	void* ptr = malloc(raw_len);
	if (ptr == NULL) return 0;
	memcpy(ptr, raw, raw_len);
	response->ptr = ptr;
	response->len = raw_len;
	return 1;
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"

	"cpa-account-pilot/internal/common"
	"cpa-account-pilot/internal/service"
)

// cliproxy_plugin_init 注册宿主回调及插件函数表。
//
//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	// The host may load the ABI without sending plugin.register immediately.
	// Initialize storage here so the encryption key exists as soon as the
	// native plugin is loaded.
	if err := service.InitializePrivateAccountStore(); err != nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(1)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

// cliproxyPluginCall 解析 ABI 请求并分发给服务层。
//
//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, common.ErrorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, err := handleMethod(C.GoString(method), requestBytes, pluginVersion, callHost)
	if err != nil {
		raw = common.ErrorEnvelope("plugin_error", err.Error())
		writeResponse(response, raw)
		return 1
	}
	if !writeResponse(response, raw) {
		return 1
	}
	return 0
}

// cliproxyPluginFree 释放返回给 CPA 宿主的内存。
//
//export cliproxyPluginFree
func cliproxyPluginFree(pointer unsafe.Pointer, length C.size_t) {
	if pointer != nil {
		C.free(pointer)
	}
	_ = length
}

// cliproxyPluginShutdown 清除已保存的宿主回调表。
//
//export cliproxyPluginShutdown
func cliproxyPluginShutdown() { C.clear_host_api() }

// callHost 序列化回调参数、调用 CPA 宿主并解包返回结果。
func callHost(method string, payload any) (json.RawMessage, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal host callback %s: %w", method, err)
	}
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	var response C.cliproxy_buffer
	var requestPtr *C.uint8_t
	if len(rawPayload) > 0 {
		payloadPtr := C.CBytes(rawPayload)
		if payloadPtr == nil {
			return nil, fmt.Errorf("allocate host callback payload %s", method)
		}
		defer C.free(payloadPtr)
		requestPtr = (*C.uint8_t)(payloadPtr)
	}
	callCode := C.call_host_api(cMethod, requestPtr, C.size_t(len(rawPayload)), &response)
	var rawResponse []byte
	if response.ptr != nil && response.len > 0 {
		rawResponse = C.GoBytes(response.ptr, C.int(response.len))
	}
	if response.ptr != nil {
		C.free_host_buffer(response.ptr, response.len)
	}
	if len(rawResponse) == 0 {
		return nil, fmt.Errorf("host callback %s returned no response, code=%d", method, int(callCode))
	}
	result, err := common.DecodeEnvelopeResult(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	if callCode != 0 {
		return nil, fmt.Errorf("host callback %s returned code=%d", method, int(callCode))
	}
	return result, nil
}

// writeResponse 将 Go 响应复制到由 CPA 管理的 ABI 输出内存。
func writeResponse(response *C.cliproxy_buffer, raw []byte) bool {
	if response == nil || len(raw) == 0 {
		return false
	}
	return C.write_plugin_response((*C.uint8_t)(unsafe.Pointer(&raw[0])), C.size_t(len(raw)), response) != 0
}
