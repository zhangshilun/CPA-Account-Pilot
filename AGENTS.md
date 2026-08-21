# CPA Account Vault

CLIProxyAPI 原生插件，用于持久化管理私有账号、账号文件、凭证状态和快速登录。

## 功能实现核心路径

插件是 CLIProxyAPI 原生插件，主要调用链如下：

```text
CLIProxyAPI
  → cliproxy_plugin_init / cliproxyPluginCall（cgo_bridge.go）
  → handleMethod（plugin.go）
  → handleManagement（management.go）
  → service 服务层或嵌入式 Web 资源
```

### 生命周期与配置

- `plugin.register` 声明插件元数据、管理能力和配置项。
- `plugin.reconfigure` 解析 `CPA_ACCOUNTS_VAULT_KEY`、`CPA_ACCOUNTS_VAULT_DIR`，并初始化账号存储。
- `CPA_ACCOUNTS_VAULT_KEY` 必须是 Base64 编码的 32 字节 AES 密钥。
- `CPA_ACCOUNTS_VAULT_DIR` 可选，默认值为 `/CLIProxyAPI/data/cpa-account-vault/`。
- 初始化存储时会创建目录、读取 JSON 记录并校验加密密钥。

### 管理接口

路由在 `plugin.go` 中注册，在 `management.go` 中分发：

| 方法 | 路径后缀 | 作用 |
| --- | --- | --- |
| `GET` | `/accounts` | 获取私有账号和 CPA 凭证关联状态 |
| `POST` | `/accounts/import` | 导入逐行 JSON 账号，最多 500 条 |
| `PUT` | `/account` | 创建或更新一个私有账号 |
| `DELETE` | `/account` | 删除一个私有账号记录 |
| `GET` | `/auth-files` | 获取脱敏后的 CPA 认证文件元数据 |
| `POST` | `/account-files` | 将导入的账号对象保存为独立 JSON 文件 |
| `GET` | `/account-files` | 读取已保存账号并解密密码 |

管理页面通过 `/index.html` 提供，由 `web/web.go` 将 `index.html`、`styles.css` 和 `app.js` 嵌入并合并为一个响应。

### 账号持久化与加密

`internal/service/private_accounts.go` 负责私有账号记录。创建、更新和导入时会校验 `account`、`email`，生成或复用 ID，加密密码并以原子方式写入 JSON 文件。

密码使用 AES-GCM 加密，磁盘字段为 `password_cipher`，不会保存明文密码。管理接口只返回 `password_set`，不会返回私有账号密码。

`internal/service/auth_files.go` 负责导入账号对象。写入时将 `password` 转为 `password_cipher`；授权读取时在内存中解密并返回给管理页面。文件使用受限权限，并通过临时文件重命名方式写入。

### CPA 凭证关联

服务层通过 `common.HostCall` 调用宿主，C 桥接实现在 `cgo_bridge.go`：

```text
service
  → host.auth.list
  → host.auth.get 获取plan_type
  → 获取脱敏后的 CPA 认证元数据
  → 按邮箱不区分大小写匹配
  → 计算每个私有账号的关联状态
```

### 前端功能

`web/app.js` 从宿主页面存储中读取 CPA API 地址和管理密钥，并使用 Bearer Token 调用管理接口。前端提供账号导入、编辑、搜索、筛选、下载、凭证状态刷新、邮箱打开和 Provider 登录功能。

Provider 登录由前端调用对应的 CPA 管理登录接口发起。插件本身不获取 OAuth Token，只负责打开返回的授权 URL；如果浏览器阻止弹窗，则复制 URL 供用户手动打开。

邮箱访问同样只在前端完成，通过账号对象中的 `mailbox_url` 打开邮箱页面。

### 数据目录注意事项

修改`CPA_ACCOUNTS_VAULT_DIR`时需要迁移原路径下文件到新路径

## 参考实现
/Volumes/FILE/Tools/CPA-Account-Pilot/事例/cpa-plugin-sub2api-balance-main

## CPA sdk和examples
/Volumes/FILE/Tools/CPA-Account-Pilot/事例/CLIProxyAPI-main

## 要求
web目录下仅静态页面，app.js仅负责调用，不能出现任何配置和魔法路径，app.js仅参考核心逻辑