// Package service contains the account-management use cases exposed by the plugin.
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cpa-account-pilot/internal/common"
	"cpa-account-pilot/internal/cpaapi"
)

var accountFilesDirectory = "/CLIProxyAPI/data/cpa-account-pilot"

var unsafeFileName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// PersistAccountFiles saves each imported account object in its own JSON file
// alongside the plugin's encrypted private-account store.
func PersistAccountFiles(raw []byte) ([]byte, error) {
	if err := accountStore.load(); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	accounts := make([]map[string]any, 0)
	for {
		var account map[string]any
		err := decoder.Decode(&account)
		if err == io.EOF {
			break
		}
		if err != nil || len(account) == 0 {
			return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "invalid_account_file", "message": "账户文件必须包含 JSON 对象"}))
		}
		accounts = append(accounts, account)
	}
	if len(accounts) == 0 {
		return common.OKEnvelope(common.JSONResponse(http.StatusBadRequest, map[string]string{"error": "empty_account_file", "message": "没有可保存的账户"}))
	}
	if err := os.MkdirAll(accountFilesDirectory, 0o700); err != nil {
		return nil, err
	}
	used := make(map[string]bool)
	for index, account := range accounts {
		if err := encryptAccountPassword(account); err != nil {
			return nil, err
		}
		name := accountFileName(account, index)
		for used[name] {
			name = fmt.Sprintf("%s-%d.json", strings.TrimSuffix(name, ".json"), index+1)
		}
		used[name] = true
		content, err := json.MarshalIndent(account, "", "  ")
		if err != nil {
			return nil, err
		}
		path := filepath.Join(accountFilesDirectory, name)
		if err := os.WriteFile(path+".tmp", append(content, '\n'), 0o600); err != nil {
			return nil, err
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			return nil, err
		}
	}
	accountStore.mu.Lock()
	accountStore.loaded = false
	accountStore.accounts = nil
	accountStore.mu.Unlock()
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"saved": len(accounts), "directory": accountFilesDirectory}))
}

// ListAccountFiles loads all user-managed account objects from the dedicated
// directory. The page uses this on startup, so all later operations target the
// persisted per-account files rather than the initially selected source file.
func ListAccountFiles() ([]byte, error) {
	entries, err := os.ReadDir(accountFilesDirectory)
	if os.IsNotExist(err) {
		return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"accounts": []any{}}))
	}
	if err != nil {
		return nil, err
	}
	accounts := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(accountFilesDirectory, entry.Name()))
		if err != nil {
			return nil, err
		}
		var account map[string]any
		if err := json.Unmarshal(raw, &account); err != nil {
			return nil, fmt.Errorf("读取账户文件 %s: %w", entry.Name(), err)
		}
		if err := decryptAccountPassword(account); err != nil {
			return nil, fmt.Errorf("解密账户文件 %s: %w", entry.Name(), err)
		}
		accounts = append(accounts, account)
	}
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"accounts": accounts, "directory": accountFilesDirectory}))
}

func encryptAccountPassword(account map[string]any) error {
	password := strings.TrimSpace(fmt.Sprint(account["password"]))
	ciphertext := strings.TrimSpace(fmt.Sprint(account["password_cipher"]))
	if password == "" || password == "<nil>" || ciphertext != "" && ciphertext != "<nil>" {
		delete(account, "password")
		return nil
	}
	encrypted, err := encryptPassword(accountStore.key, password)
	if err != nil {
		return err
	}
	account["password_cipher"] = encrypted
	delete(account, "password")
	return nil
}

func decryptAccountPassword(account map[string]any) error {
	ciphertext := strings.TrimSpace(fmt.Sprint(account["password_cipher"]))
	if ciphertext == "" || ciphertext == "<nil>" {
		return nil
	}
	password, err := decryptPassword(accountStore.key, ciphertext)
	if err != nil {
		return err
	}
	account["password"] = password
	delete(account, "password_cipher")
	return nil
}

func accountFileName(account map[string]any, index int) string {
	value := strings.TrimSpace(fmt.Sprint(account["email"]))
	if value == "" {
		value = strings.TrimSpace(fmt.Sprint(account["account"]))
	}
	value = strings.Trim(unsafeFileName.ReplaceAllString(value, "-"), "-.")
	if value == "" || value == "<nil>" {
		value = fmt.Sprintf("account-%d", index+1)
	}
	return value + ".json"
}

// ListAuthFiles exposes redacted CPA authentication-file metadata to the
// management page, so imported account records can be associated by email.
func ListAuthFiles(hostCall common.HostCall) ([]byte, error) {
	files, err := listHostAuthFiles(hostCall)
	if err != nil {
		return nil, err
	}
	entries := make([]map[string]any, 0, len(files))
	for _, file := range files {
		status := strings.TrimSpace(file.Status)
		if file.Disabled {
			status = "disabled"
		} else if file.Unavailable {
			status = "unavailable"
		} else if status == "" {
			status = "active"
		}
		entries = append(entries, map[string]any{
			"id": file.ID, "auth_index": file.AuthIndex, "account": file.Account,
			"email": file.Email, "disabled": file.Disabled, "status": status, "status_message": file.StatusMessage, "updated_at": file.UpdatedAt,
		})
	}
	return common.OKEnvelope(common.JSONResponse(http.StatusOK, map[string]any{"files": entries}))
}

// listHostAuthFiles reads only the redacted metadata exposed by CPA.
func listHostAuthFiles(hostCall common.HostCall) ([]cpaapi.HostAuthFileEntry, error) {
	raw, err := hostCall(cpaapi.MethodHostAuthList, map[string]any{})
	if err != nil {
		return nil, err
	}
	var result cpaapi.HostAuthListResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode host.auth.list result: %w", err)
	}
	return result.Files, nil
}
