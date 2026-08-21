# CPA Account Vault

CLIProxyAPI 原生插件，用于管理账号文件、复制账号密码、查看凭证状态和快速登录。

## 使用方法

### 1. 构建插件

```bash
make build
```

构建产物位于 `dist/`：

```text
dist/cpa-account-vault.dylib   # macOS
dist/cpa-account-vault.so      # Linux
dist/cpa-account-vault.dll     # Windows
```

将对应平台的插件文件放入 CLIProxyAPI 插件目录并启动或重新加载 CLIProxyAPI。

### 2. 打开管理页面

通过 CLIProxyAPI 管理页面进入 CPA Account Vault，或访问：

```text
/v0/resource/plugins/cpa-account-vault/index.html
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
/CLIProxyAPI/data/cpa-account-vault/
```

每个账号单独保存为 JSON 文件，密码使用插件可视化配置项 `CPA_ACCOUNTS_VAULT_KEY` 提供的 32 字节主密钥加密保存。该值必须是标准 Base64 编码。`CPA_ACCOUNTS_VAULT_KEY` 为必填项，`CPA_ACCOUNTS_VAULT_DIR` 可选，默认值为 `/CLIProxyAPI/data/cpa-account-vault/`。

生成密钥示例：

```bash
openssl rand -base64 32
```

将命令输出的密钥填写到 CLIProxyAPI 的插件配置页面。配置完成后请一直使用同一个密钥，否则已有账号密码无法解密。

## LICENSE

本项目基于 [MIT License](LICENSE) 开源。
