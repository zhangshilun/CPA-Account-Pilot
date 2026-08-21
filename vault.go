package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type privateAccount struct {
	ID             string    `json:"id"`
	Account        string    `json:"account"`
	Email          string    `json:"email"`
	Provider       string    `json:"provider,omitempty"`
	PasswordCipher string    `json:"password_cipher,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type accountInput struct {
	ID       string `json:"id,omitempty"`
	Account  string `json:"account"`
	Email    string `json:"email"`
	Provider string `json:"provider,omitempty"`
	Password string `json:"password,omitempty"`
}

var accountStore struct {
	sync.Mutex
	directory string
	key       []byte
	accounts  map[string]privateAccount
}

const (
	jsonFileExtension                   = ".json"
	temporaryFilePattern                = ".vault-*.tmp"
	passwordCipherField                 = "password_cipher"
	passwordField                       = "password"
	maximumImportedAccounts             = 500
	directoryPermissions    os.FileMode = 0o700
	filePermissions         os.FileMode = 0o600
	aesKeyLength                        = 32
)

var unsafeFileName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// VaultConfig 保存运行时使用的 Vault 路径和 AES 密钥。
type VaultConfig struct {
	Directory string
	Key       []byte
}

// runtimeConfig 保护当前生效的 Vault 配置。
var runtimeConfig struct {
	sync.RWMutex
	value              VaultConfig
	configured         bool
	configurationError string
}

// ConfigureVault 校验密钥、初始化目录，并在目录变化时迁移旧 Vault。
func ConfigureVault(keyBase64, directory string, defaultDirectory string) error {
	key, err := decodeVaultKey(keyBase64)
	if err != nil {
		return err
	}
	if strings.TrimSpace(directory) == "" {
		directory = defaultDirectory
	}
	directory, err = filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return fmt.Errorf("解析 Vault 目录: %w", err)
	}

	runtimeConfig.Lock()
	defer runtimeConfig.Unlock()
	if runtimeConfig.configured && runtimeConfig.value.Directory != directory {
		if err := validateVaultKey(runtimeConfig.value.Directory, key); err != nil {
			return err
		}
		if err := migrateVault(runtimeConfig.value.Directory, directory); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(directory, directoryPermissions); err != nil {
		return fmt.Errorf("创建 Vault 目录: %w", err)
	}
	if err := os.Chmod(directory, directoryPermissions); err != nil {
		return fmt.Errorf("设置 Vault 目录权限: %w", err)
	}
	if err := validateVaultKey(directory, key); err != nil {
		return err
	}
	runtimeConfig.value = VaultConfig{Directory: directory, Key: append([]byte(nil), key...)}
	runtimeConfig.configured = true
	runtimeConfig.configurationError = ""
	return nil
}

// SetVaultConfigurationError 清除当前 Vault 并保存可安全显示给管理页面的配置错误。
func SetVaultConfigurationError(message string) {
	runtimeConfig.Lock()
	runtimeConfig.value = VaultConfig{}
	runtimeConfig.configured = false
	runtimeConfig.configurationError = strings.TrimSpace(message)
	runtimeConfig.Unlock()
}

// VaultConfigurationStatus 返回管理页面用于提示用户的 Vault 配置状态。
func VaultConfigurationStatus() map[string]any {
	runtimeConfig.RLock()
	defer runtimeConfig.RUnlock()
	message := runtimeConfig.configurationError
	if !runtimeConfig.configured && message == "" {
		message = "请在 CPA 插件配置中填写 CPA_ACCOUNTS_VAULT_KEY。"
	}
	return map[string]any{"configured": runtimeConfig.configured, "message": message}
}

/*
// ListPrivateAccounts 返回账号的非敏感视图及 CPA 凭证关联状态。
func ListPrivateAccounts(hostCall HostCall) ([]byte, error) {
	if err := loadAccountStore(); err != nil {
		return nil, err
	}
	credentials, _ := fetchAuthMetadata(hostCall)
	byEmail := make(map[string]authMetadata, len(credentials))
	for _, credential := range credentials {
		if credential.Email != "" {
			byEmail[strings.ToLower(credential.Email)] = credential
		}
	}
	accountStore.Lock()
	items := make([]map[string]any, 0, len(accountStore.accounts))
	for _, account := range accountStore.accounts {
		view := accountView(account)
		if credential, found := byEmail[strings.ToLower(strings.TrimSpace(account.Email))]; found {
			view["auth_index"], view["link_status"] = credential.AuthIndex, credential.Status
		} else {
			view["link_status"] = "unlinked"
		}
		items = append(items, view)
	}
	accountStore.Unlock()
	sort.Slice(items, func(i, j int) bool { return fmt.Sprint(items[i]["updated_at"]) > fmt.Sprint(items[j]["updated_at"]) })
	return OK(JSON(http.StatusOK, map[string]any{"accounts": items, "total": len(items)}))
}

// UpsertPrivateAccount 创建或更新一条私有账号记录。
func UpsertPrivateAccount(raw []byte) ([]byte, error) {
	var input accountInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return badRequest("invalid_account", "账号必须是 JSON 对象")
	}
	if err := validateAccount(input); err != nil {
		return badRequest("invalid_account", err.Error())
	}
	if err := loadAccountStore(); err != nil {
		return nil, err
	}
	accountStore.Lock()
	defer accountStore.Unlock()
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newAccountID()
	}
	account, now := accountStore.accounts[id], time.Now().UTC()
	if account.ID == "" {
		account.ID, account.CreatedAt = id, now
	}
	account.Account, account.Email, account.Provider, account.UpdatedAt = strings.TrimSpace(input.Account), strings.TrimSpace(input.Email), strings.TrimSpace(input.Provider), now
	if input.Password != "" {
		ciphertext, err := encryptPassword(accountStore.key, input.Password)
		if err != nil {
			return nil, err
		}
		account.PasswordCipher = ciphertext
	}
	if err := writeJSONAtomically(filepath.Join(accountStore.directory, id+jsonFileExtension), account); err != nil {
		return nil, err
	}
	accountStore.accounts[id] = account
	return OK(JSON(http.StatusOK, map[string]any{"account": accountView(account)}))
}

// ImportPrivateAccounts 导入至多 500 条逐行 JSON 格式的私有账号。
func ImportPrivateAccounts(raw []byte) ([]byte, error) {
	decoder, inputs := json.NewDecoder(bytes.NewReader(raw)), make([]accountInput, 0)
	for {
		var input accountInput
		err := decoder.Decode(&input)
		if err == io.EOF {
			break
		}
		if err != nil {
			return badRequest("invalid_import", "导入内容必须是逐行 JSON 对象")
		}
		if err := validateAccount(input); err != nil {
			return badRequest("invalid_import", err.Error())
		}
		inputs = append(inputs, input)
		if len(inputs) > maximumImportedAccounts {
			return badRequest("invalid_import", "导入账号不能超过 500 条")
		}
	}
	if len(inputs) == 0 {
		return badRequest("invalid_import", "至少需要一条账号")
	}
	for _, input := range inputs {
		payload, _ := json.Marshal(input)
		if _, err := UpsertPrivateAccount(payload); err != nil {
			return nil, err
		}
	}
	return OK(JSON(http.StatusOK, map[string]int{"imported": len(inputs)}))
}

// DeletePrivateAccount 删除插件私有账号，不影响 CPA 原始凭证。
func DeletePrivateAccount(raw []byte) ([]byte, error) {
	var request struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &request); err != nil || strings.TrimSpace(request.ID) == "" {
		return badRequest("invalid_account", "缺少账号 ID")
	}
	if err := loadAccountStore(); err != nil {
		return nil, err
	}
	accountStore.Lock()
	defer accountStore.Unlock()
	if _, found := accountStore.accounts[request.ID]; !found {
		return OK(JSON(http.StatusNotFound, map[string]string{"error": "account_not_found"}))
	}
	if err := os.Remove(filepath.Join(accountStore.directory, request.ID+jsonFileExtension)); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("删除私有账号: %w", err)
	}
	delete(accountStore.accounts, request.ID)
	return OK(JSON(http.StatusOK, map[string]string{"status": "deleted"}))
}

// PersistAccountFiles 将账号对象分别保存为单独的加密 JSON 文件。
*/

func PersistAccountFiles(raw []byte) ([]byte, error) {
	config, err := currentVaultConfig()
	if err != nil {
		return nil, err
	}
	accounts, err := decodeAccountObjects(raw)
	if err != nil {
		return badRequest("invalid_account_file", err.Error())
	}
	directory := config.Directory
	if err := os.MkdirAll(directory, directoryPermissions); err != nil {
		return nil, fmt.Errorf("创建账号文件目录: %w", err)
	}
	if err := os.Chmod(directory, directoryPermissions); err != nil {
		return nil, fmt.Errorf("设置账号文件目录权限: %w", err)
	}
	for index, account := range accounts {
		if password, found := stringValue(account[passwordField]); found && password != "" {
			ciphertext, err := encryptPassword(config.Key, password)
			if err != nil {
				return nil, err
			}
			account[passwordCipherField] = ciphertext
		}
		delete(account, passwordField)
		if err := writeJSONAtomically(filepath.Join(directory, uniqueAccountFileName(account, index)), account); err != nil {
			return nil, err
		}
	}
	return OK(JSON(http.StatusOK, map[string]int{"saved": len(accounts)}))
}

// ListAccountFiles 只在已鉴权的管理响应中于内存内解密账号密码。
func ListAccountFiles() ([]byte, error) {
	config, err := currentVaultConfig()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(config.Directory)
	if os.IsNotExist(err) {
		return OK(JSON(http.StatusOK, map[string]any{"accounts": []any{}}))
	}
	if err != nil {
		return nil, fmt.Errorf("读取账号文件: %w", err)
	}
	accounts := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), jsonFileExtension) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(config.Directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("读取账号文件 %s: %w", entry.Name(), err)
		}
		var account map[string]any
		if err := json.Unmarshal(raw, &account); err != nil {
			return nil, fmt.Errorf("解析账号文件 %s: %w", entry.Name(), err)
		}
		if ciphertext, found := stringValue(account[passwordCipherField]); found && ciphertext != "" {
			password, err := decryptPassword(config.Key, ciphertext)
			if err != nil {
				return nil, fmt.Errorf("解密账号文件 %s: %w", entry.Name(), err)
			}
			account[passwordField] = password
			delete(account, passwordCipherField)
		}
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return fmt.Sprint(accounts[i]["email"]) < fmt.Sprint(accounts[j]["email"]) })
	return OK(JSON(http.StatusOK, map[string]any{"accounts": accounts}))
}

// ListAuthFiles 返回经过白名单过滤且不含凭证内容的 CPA 认证文件信息。
func ListAuthFiles(hostCall HostCall) ([]byte, error) {
	files, err := fetchAuthMetadata(hostCall)
	if err != nil {
		return nil, err
	}
	entries := make([]map[string]any, 0, len(files))
	for _, file := range files {
		entries = append(entries, map[string]any{"id": file.ID, "auth_index": file.AuthIndex, "name": file.Name, "email": file.Email, "provider": file.Provider, "status": file.Status, "status_message": file.StatusMessage, "disabled": file.Disabled, "unavailable": file.Unavailable, "websockets": file.Websockets, "plan_type": file.PlanType})
	}
	return OK(JSON(http.StatusOK, map[string]any{"files": entries}))
}

// authMetadata 表示关联私有账号时需要的凭证元数据。
type authMetadata struct {
	ID, AuthIndex, Name, Email, Provider, Status, StatusMessage, PlanType string
	Disabled, Unavailable, Websockets                                     bool
}

// fetchAuthMetadata 依次请求 CPA 的凭证列表与详情，并只提取允许公开字段。
func fetchAuthMetadata(hostCall HostCall) ([]authMetadata, error) {
	if hostCall == nil {
		return nil, fmt.Errorf("CPA 宿主回调不可用")
	}
	raw, err := hostCall("host.auth.list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("读取 CPA 凭证列表: %w", err)
	}
	var listed HostAuthListResponse
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("解析 CPA 凭证列表: %w", err)
	}
	files := make([]authMetadata, 0, len(listed.Files))
	for _, entry := range listed.Files {
		file := authMetadata{ID: entry.ID, AuthIndex: entry.AuthIndex, Name: entry.Name, Email: entry.Email, Provider: entry.Provider, Status: entry.Status, StatusMessage: entry.StatusMessage, Disabled: entry.Disabled, Unavailable: entry.Unavailable, Websockets: entry.Websockets}
		if entry.AuthIndex != "" {
			result, getErr := hostCall("host.auth.get", HostAuthGetRequest{AuthIndex: entry.AuthIndex})
			if getErr == nil {
				var response HostAuthGetResponse
				if json.Unmarshal(result, &response) == nil {
					var credential struct {
						Email    string `json:"email"`
						PlanType string `json:"plan_type"`
						Provider string `json:"provider"`
					}
					if json.Unmarshal(response.JSON, &credential) == nil {
						if credential.Email != "" {
							file.Email = credential.Email
						}
						if credential.PlanType != "" {
							file.PlanType = credential.PlanType
						}
						if credential.Provider != "" {
							file.Provider = credential.Provider
						}
					}
				}
			}
		}
		if file.Disabled {
			file.Status = "disabled"
		} else if file.Unavailable || file.Email == "" {
			file.Status = "unavailable"
		} else if file.Status == "" {
			file.Status = "active"
		}
		files = append(files, file)
	}
	return files, nil
}

// currentVaultConfig 返回已复制的当前配置，防止调用者修改内存中的密钥。
func currentVaultConfig() (VaultConfig, error) {
	runtimeConfig.RLock()
	defer runtimeConfig.RUnlock()
	if !runtimeConfig.configured {
		message := runtimeConfig.configurationError
		if message == "" {
			message = "CPA_ACCOUNTS_VAULT_KEY 尚未配置"
		}
		return VaultConfig{}, errors.New(message)
	}
	return VaultConfig{Directory: runtimeConfig.value.Directory, Key: append([]byte(nil), runtimeConfig.value.Key...)}, nil
}

// decodeVaultKey 将 Base64 输入校验为 AES-256 所需的 32 字节密钥。
func decodeVaultKey(value string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(key) != aesKeyLength {
		return nil, errors.New("CPA_ACCOUNTS_VAULT_KEY 必须是 Base64 编码的 32 字节 AES 密钥")
	}
	return key, nil
}

// validateVaultKey 验证提供的密钥可以解密现有 Vault 的所有密码字段。
func validateVaultKey(directory string, key []byte) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("读取现有 Vault: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), jsonFileExtension) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("读取现有 Vault 文件: %w", err)
		}
		var record map[string]any
		if err := json.Unmarshal(raw, &record); err != nil {
			return fmt.Errorf("解析现有 Vault 文件: %w", err)
		}
		if ciphertext, found := stringValue(record[passwordCipherField]); found && ciphertext != "" {
			if _, err := decryptPassword(key, ciphertext); err != nil {
				return errors.New("CPA_ACCOUNTS_VAULT_KEY 无法解密现有 Vault 数据")
			}
		}
	}
	return nil
}

// migrateVault 将旧 Vault 移动到新的空目录，保证配置路径变化不会丢失数据。
func migrateVault(source, destination string) error {
	if source == destination {
		return nil
	}
	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查现有 Vault: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("现有 Vault 路径不是目录: %s", source)
	}
	if _, err := os.Stat(destination); err == nil {
		entries, readErr := os.ReadDir(destination)
		if readErr != nil {
			return fmt.Errorf("读取目标 Vault: %w", readErr)
		}
		if len(entries) != 0 {
			return errors.New("无法迁移 Vault：目标目录非空")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查目标 Vault: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), directoryPermissions); err != nil {
		return fmt.Errorf("创建目标父目录: %w", err)
	}
	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("迁移 Vault: %w", err)
	}
	return nil
}

/*
// resetAccountStore 清空与上一套 Vault 配置关联的内存缓存。
func resetAccountStore() {
	accountStore.Lock()
	defer accountStore.Unlock()
	accountStore.directory, accountStore.key, accountStore.accounts = "", nil, nil
}

// loadAccountStore 保留旧实现以兼容编译；私有账号路由已不再注册。
func loadAccountStore() error {
	config, err := currentVaultConfig()
	if err != nil {
		return err
	}
	accountStore.Lock()
	defer accountStore.Unlock()
	if accountStore.accounts != nil && accountStore.directory == config.Directory {
		return nil
	}
	directory := config.Directory
	if err := os.MkdirAll(directory, directoryPermissions); err != nil {
		return fmt.Errorf("创建私有账号目录: %w", err)
	}
	if err := os.Chmod(directory, directoryPermissions); err != nil {
		return fmt.Errorf("设置私有账号目录权限: %w", err)
	}
	accounts := make(map[string]privateAccount)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("读取私有账号: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), jsonFileExtension) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("读取私有账号: %w", err)
		}
		var account privateAccount
		if err := json.Unmarshal(raw, &account); err != nil {
			return fmt.Errorf("解析私有账号: %w", err)
		}
		if account.ID == "" || account.Account == "" || account.Email == "" {
			return fmt.Errorf("私有账号文件不完整: %s", entry.Name())
		}
		accounts[account.ID] = account
	}
	accountStore.directory, accountStore.key, accountStore.accounts = directory, config.Key, accounts
	return nil
}

// accountView 创建绝不包含密码密文的私有账号响应对象。
func accountView(account privateAccount) map[string]any {
	return map[string]any{"id": account.ID, "account": account.Account, "email": account.Email, "provider": account.Provider, "password_set": account.PasswordCipher != "", "created_at": account.CreatedAt, "updated_at": account.UpdatedAt}
}

// validateAccount 检查业务必填字段。
func validateAccount(input accountInput) error {
	if strings.TrimSpace(input.Account) == "" || strings.TrimSpace(input.Email) == "" {
		return errors.New("账号和邮箱不能为空")
	}
	return nil
}

// newAccountID 生成稳定的随机账号 ID。
func newAccountID() string {
	raw := make([]byte, randomAccountIDBytes)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return fmt.Sprintf("%s%d", accountIDPrefix, time.Now().UnixNano())
	}
	return accountIDPrefix + hex.EncodeToString(raw)
}
*/

// badRequest 创建统一的 400 管理接口响应。
func badRequest(code, message string) ([]byte, error) {
	return OK(JSON(http.StatusBadRequest, map[string]string{"error": code, "message": message}))
}

// writeJSONAtomically 以临时文件和重命名方式安全写入受限权限 JSON 文件。
func writeJSONAtomically(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), temporaryFilePattern)
	if err != nil {
		return fmt.Errorf("创建临时 Vault 文件: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(filePermissions); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(raw, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("替换 Vault 文件: %w", err)
	}
	return os.Chmod(path, filePermissions)
}

// decodeAccountObjects 读取逐行 JSON 格式的账号对象。
func decodeAccountObjects(raw []byte) ([]map[string]any, error) {
	objects := make([]map[string]any, 0)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &objects); err != nil {
			return nil, errors.New("账号文件格式无效，请使用逐行 JSON 对象或 JSON 对象数组")
		}
		for _, account := range objects {
			if len(account) == 0 {
				return nil, errors.New("账号文件必须包含有效的 JSON 对象")
			}
		}
		if len(objects) > maximumImportedAccounts {
			return nil, errors.New("账号文件不能超过 500 条")
		}
		if len(objects) == 0 {
			return nil, errors.New("至少需要一个账号文件")
		}
		return objects, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var account map[string]any
		err := decoder.Decode(&account)
		if err == io.EOF {
			break
		}
		if err != nil || len(account) == 0 {
			return nil, errors.New("账号文件必须是逐行 JSON 对象")
		}
		objects = append(objects, account)
		if len(objects) > maximumImportedAccounts {
			return nil, errors.New("账号文件不能超过 500 条")
		}
	}
	if len(objects) == 0 {
		return nil, errors.New("至少需要一个账号文件")
	}
	return objects, nil
}

// uniqueAccountFileName 依据邮箱或账号生成文件名，同名文件由原子写入覆盖。
func uniqueAccountFileName(account map[string]any, index int) string {
	base, _ := stringValue(account["email"])
	if base == "" {
		base, _ = stringValue(account["account"])
	}
	base = strings.Trim(unsafeFileName.ReplaceAllString(base, "-"), "-.")
	if base == "" {
		base = fmt.Sprintf("%d", index+1)
	}
	return base + jsonFileExtension
}

// stringValue 安全地读取 map 中的非空字符串字段。
func stringValue(value any) (string, bool) {
	text, ok := value.(string)
	return strings.TrimSpace(text), ok
}

// encryptPassword 使用 AES-GCM 加密密码并返回 Base64 密文。
func encryptPassword(key []byte, password string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(password), nil)), nil
}

// decryptPassword 使用 AES-GCM 解密 Base64 密文。
func decryptPassword(key []byte, ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("密文不完整")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
