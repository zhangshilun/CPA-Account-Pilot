package service

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cpa-account-pilot/internal/cpaapi"
)

func TestInitializePrivateAccountStoreKeepsExistingKey(t *testing.T) {
	dir := t.TempDir()
	previousDir := accountFilesDirectory
	accountFilesDirectory = dir
	t.Cleanup(func() { accountFilesDirectory = previousDir })
	accountStore = privateAccountStore{}

	if err := InitializePrivateAccountStore(); err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	keyPath := filepath.Join(dir, "private_accounts.key")
	firstKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read generated key: %v", err)
	}
	if len(firstKey) != 32 {
		t.Fatalf("generated key length = %d, want 32", len(firstKey))
	}

	// Reset the in-memory store to simulate a later plugin process startup.
	accountStore = privateAccountStore{}
	if err := InitializePrivateAccountStore(); err != nil {
		t.Fatalf("reinitialize store: %v", err)
	}
	secondKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read existing key: %v", err)
	}
	if !bytes.Equal(firstKey, secondKey) {
		t.Fatal("existing key was replaced during reinitialization")
	}
}

func TestAccountFilesUseKeyAndIndividualJSONFiles(t *testing.T) {
	dir := t.TempDir()
	previousDir := accountFilesDirectory
	accountFilesDirectory = dir
	t.Cleanup(func() { accountFilesDirectory = previousDir })
	accountStore = privateAccountStore{}

	if _, err := PersistAccountFiles([]byte(`{"email":"user@example.com","password":"secret"}`)); err != nil {
		t.Fatalf("persist account file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "private_accounts.json")); !os.IsNotExist(err) {
		t.Fatalf("private_accounts.json should not exist")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read account directory: %v", err)
	}
	var accountPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			accountPath = filepath.Join(dir, entry.Name())
			break
		}
	}
	if accountPath == "" {
		t.Fatal("no individual account JSON file was created")
	}
	raw, err := os.ReadFile(accountPath)
	if err != nil {
		t.Fatalf("read account file: %v", err)
	}
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), `"password"`) {
		t.Fatalf("account file contains plaintext password: %s", raw)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode account file: %v", err)
	}
	if strings.TrimSpace(stored["password_cipher"].(string)) == "" {
		t.Fatal("account file has no password cipher")
	}

	response, err := ListAccountFiles()
	if err != nil {
		t.Fatalf("list account files: %v", err)
	}
	var envelope struct {
		Result cpaapi.ManagementResponse `json:"result"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	var body struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.Unmarshal(envelope.Result.Body, &body); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	if len(body.Accounts) != 1 || body.Accounts[0]["password"] != "secret" {
		t.Fatalf("list response did not decrypt password: %#v", body.Accounts)
	}
}
