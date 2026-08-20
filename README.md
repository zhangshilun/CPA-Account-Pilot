# CPA Account Pilot

CLIProxyAPI 原生插件，用于管理账号文件、复制账号密码、查看凭证状态和快速登录。

## 使用方法

### 1. 构建插件

```bash
make build
```

构建产物位于 `dist/`：

```text
dist/cpa-account-pilot.dylib   # macOS
dist/cpa-account-pilot.so      # Linux
dist/cpa-account-pilot.dll     # Windows
```

将对应平台的插件文件放入 CLIProxyAPI 插件目录并启动或重新加载 CLIProxyAPI。

### 2. 打开管理页面

通过 CLIProxyAPI 管理页面进入 CPA Account Pilot，或访问：

```text
/v0/resource/plugins/cpa-account-pilot/index.html
```

管理页面支持：

- 导入和管理账号文件
- 搜索、筛选和删除账号
- 复制账号和解密后的密码
- 下载解密后的账号 JSON/TAR 文件
- 查看账号与 CPA 凭证的关联状态
- 打开账号登录页面

### 3. 账号文件

账户文件保存在：

```text
/CLIProxyAPI/data/cpa-account-pilot/
```

每个账号单独保存为 JSON 文件，密码使用自动生成的 `private_accounts.key` 加密保存。首次加载插件时会自动创建密钥；已有密钥不会被覆盖。

请勿删除 `private_accounts.key`，否则已保存的密码无法解密。

## LICENSE

本项目基于 [MIT License](LICENSE) 开源。
