// 本文件负责私有账号的加密持久化、导入、删除和凭证关联状态计算。
package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cpa-account-pilot/internal/common"
	"cpa-account-pilot/internal/cpaapi"
)

type privateAccountRecord struct {
	ID             string    `json:"id"`
	Account        string    `json:"account"`
	Email          string    `json:"email"`
	Provider       string    `json:"provider,omitempty"`
	PasswordCipher string    `json:"password_cipher,omitempty"`
	AuthIndex      string    `json:"auth_index,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	FileName       string    `json:"-"`
}

type privateAccountInput struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Provider string `json:"provider,omitempty"`
}

type privateAccountView struct {
	ID          string    `json:"id"`
	Account     string    `json:"account"`
	Email       string    `json:"email"`
	Provider    string    `json:"provider,omitempty"`
	PasswordSet bool      `json:"password_set"`
	AuthIndex   string    `json:"auth_index,omitempty"`
	LinkStatus  string    `json:"link_status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type privateAccountStore struct {
	mu       sync.Mutex
	dir      string
	accounts map[string]privateAccountRecord
	key      []byte
	loaded   bool
}

var accountStore privateAccountStore

// InitializePrivateAccountStore creates the private-account data directory and
// encryption key when the plugin is registered, before any account operation.
func InitializePrivateAccountStore() error {
	return accountStore.load()
}

// ListPrivateAccounts 返回私有账号列表，并根据凭证元数据计算关联状态。
func ListPrivateAccounts(hostCall common.HostCall) ([]byte, error) {
	if err := accountStore.load(); err != nil {
		return nil, err
	}
	files, _ := listHostAuthFiles(hostCall)
	accountStore.mu.Lock()
	defer accountStore.mu.Unlock()
	items := make([]privateAccountView, 0, len(accountStore.accounts))
	for _, record := range accountStore.accounts {
		items = append(items, accountView(record, files))
	}
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"accounts": items, "total": len(items), "retrieved_at": time.Now().UTC()}))
}

// UpsertPrivateAccount 创建或更新单个私有账号。
func UpsertPrivateAccount(raw []byte) ([]byte, error) {
	var input privateAccountInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "invalid_account", "message": "账号数据格式错误"}))
	}
	if strings.TrimSpace(input.Account) == "" || strings.TrimSpace(input.Email) == "" {
		return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "account_fields_required", "message": "账号和邮箱不能为空"}))
	}
	if err := accountStore.load(); err != nil {
		return nil, err
	}
	accountStore.mu.Lock()
	defer accountStore.mu.Unlock()
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newAccountID()
	}
	record := accountStore.accounts[id]
	now := time.Now().UTC()
	if record.ID == "" {
		record.ID, record.CreatedAt = id, now
	}
	if record.FileName == "" {
		record.FileName = "account-" + id + ".json"
	}
	record.Account, record.Email, record.Provider, record.UpdatedAt = strings.TrimSpace(input.Account), strings.TrimSpace(input.Email), strings.TrimSpace(input.Provider), now
	if input.Password != "" {
		ciphertext, err := encryptPassword(accountStore.key, input.Password)
		if err != nil {
			return nil, err
		}
		record.PasswordCipher = ciphertext
	}
	accountStore.accounts[id] = record
	if err := accountStore.persistRecordLocked(record); err != nil {
		return nil, err
	}
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"account": privateAccountView{ID: id, Account: record.Account, Email: record.Email, Provider: record.Provider, PasswordSet: record.PasswordCipher != "", LinkStatus: "未匹配 CAP", CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}}))
}

// ImportPrivateAccounts imports one JSON object after another into the plugin's
// private account file. It never writes CPA authentication files.
func ImportPrivateAccounts(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	inputs := make([]privateAccountInput, 0, 16)
	for {
		var input privateAccountInput
		err := decoder.Decode(&input)
		if err == io.EOF {
			break
		}
		if err != nil {
			return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "invalid_import", "message": "导入内容必须是逐行 JSON 对象，不能使用数组"}))
		}
		inputs = append(inputs, input)
		if len(inputs) > 500 {
			return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "invalid_import_count", "message": "导入数量不能超过 500 条"}))
		}
	}
	if len(inputs) == 0 || len(inputs) > 500 {
		return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "invalid_import_count", "message": "导入数量必须在 1 到 500 条之间"}))
	}
	for _, input := range inputs {
		if strings.TrimSpace(input.Account) == "" || strings.TrimSpace(input.Email) == "" {
			return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "account_fields_required", "message": "每条账号都必须包含账号和邮箱"}))
		}
	}
	for _, input := range inputs {
		payload, _ := json.Marshal(input)
		if _, err := UpsertPrivateAccount(payload); err != nil {
			return nil, err
		}
	}
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"imported": len(inputs)}))
}

// DeletePrivateAccount 删除插件私有记录，不删除 CPA 凭证。
func DeletePrivateAccount(raw []byte) ([]byte, error) {
	var request struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &request); err != nil || strings.TrimSpace(request.ID) == "" {
		return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "account_id_required", "message": "缺少账号 ID"}))
	}
	if err := accountStore.load(); err != nil {
		return nil, err
	}
	accountStore.mu.Lock()
	defer accountStore.mu.Unlock()
	record, ok := accountStore.accounts[request.ID]
	if !ok {
		return common.OKEnvelope(common.JSONResponse(http.StatusNotFound, map[string]string{"error": "account_not_found", "message": "未找到私有账号"}))
	}
	delete(accountStore.accounts, request.ID)
	if err := os.Remove(filepath.Join(accountStore.dir, record.FileName)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]string{"status": "ok"}))
}

func accountView(record privateAccountRecord, files []cpaapi.HostAuthFileEntry) privateAccountView {
	status := "未匹配 CAP"
	for _, file := range files {
		if strings.EqualFold(strings.TrimSpace(file.Email), strings.TrimSpace(record.Email)) && strings.TrimSpace(record.Email) != "" {
			status, record.AuthIndex = "CAP 已关联", file.AuthIndex
			if file.Disabled {
				status = "CAP 凭证已禁用"
			} else if file.Unavailable {
				status = "CAP 凭证不可用"
			}
			break
		}
	}
	return privateAccountView{ID: record.ID, Account: record.Account, Email: record.Email, Provider: record.Provider, PasswordSet: record.PasswordCipher != "", AuthIndex: record.AuthIndex, LinkStatus: status, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func (s *privateAccountStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return nil
	}
	dir := accountFilesDirectory
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	s.dir = dir
	keyPath := filepath.Join(dir, "private_accounts.key")
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = io.ReadFull(rand.Reader, key); err != nil {
			return err
		}
		if err = os.WriteFile(keyPath, key, 0o600); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if len(key) != 32 {
		return errors.New("私有账号主密钥长度无效")
	}
	s.key = key
	s.accounts = make(map[string]privateAccountRecord)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return readErr
		}
		var record privateAccountRecord
		if json.Unmarshal(raw, &record) != nil || strings.TrimSpace(record.Email) == "" {
			continue
		}
		if strings.TrimSpace(record.ID) == "" {
			record.ID = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}
		record.FileName = entry.Name()
		s.accounts[record.ID] = record
	}
	s.loaded = true
	return nil
}

func (s *privateAccountStore) persistRecordLocked(record privateAccountRecord) error {
	name := record.FileName
	if name == "" {
		name = "account-" + record.ID + ".json"
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, name+".tmp")
	path := filepath.Join(s.dir, name)
	if err = os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

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
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(password), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// decryptPassword 仅用于插件内部读取已加密的敏感配置，绝不返回给管理 API。
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
		return "", errors.New("加密数据不完整")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func newAccountID() string {
	raw := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return fmt.Sprintf("account-%d", time.Now().UnixNano())
	}
	return "account-" + hex.EncodeToString(raw)
}
