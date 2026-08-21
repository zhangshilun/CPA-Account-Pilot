// Command version 从插件注册表中读取指定插件的发布版本。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// registry 表示本项目使用的 registry.json 顶层结构。
type registry struct {
	Plugins []plugin `json:"plugins"`
}

// plugin 表示读取发布版本所需的插件注册表字段。
type plugin struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// main 读取命令行参数并输出匹配插件的版本号。
func main() {
	registryPath := flag.String("registry", "registry.json", "插件注册表路径")
	pluginID := flag.String("plugin", "", "插件 ID")
	flag.Parse()
	if *pluginID == "" {
		fail("缺少 -plugin 参数")
	}
	raw, err := os.ReadFile(*registryPath)
	if err != nil {
		fail("读取注册表失败: %v", err)
	}
	var value registry
	if err := json.Unmarshal(raw, &value); err != nil {
		fail("解析注册表失败: %v", err)
	}
	for _, item := range value.Plugins {
		if item.ID == *pluginID && item.Version != "" {
			fmt.Print(item.Version)
			return
		}
	}
	fail("注册表中未找到插件 %q 的版本", *pluginID)
}

// fail 输出错误并以失败状态退出，供 Makefile 阻止错误构建。
func fail(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
