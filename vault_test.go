package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVaultEncryptDecrypt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cfa-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "my-secret-password"

	entries := []VaultEntry{
		{
			Name:      "GitHub:alice",
			Secret:    "JBSWY3DPEHPK3PXP",
			Algorithm: "SHA1",
			Digits:    6,
			Period:    30,
		},
		{
			Name:      "Google:bob",
			Secret:    "MZXW6YTBOI",
			Algorithm: "SHA256",
			Digits:    8,
			Period:    60,
		},
	}

	// Save
	err = SaveVault(vaultPath, entries, password)
	if err != nil {
		t.Fatalf("failed to save vault: %v", err)
	}

	// Verify file is created and check permissions on Unix systems
	info, err := os.Stat(vaultPath)
	if err != nil {
		t.Fatalf("vault file not created: %v", err)
	}
	if runtime.GOOS != "windows" {
		perms := info.Mode().Perm()
		if perms != 0600 {
			t.Errorf("expected permissions 0600, got: %o", perms)
		}
	}

	// Load with correct password
	loaded, err := LoadVault(vaultPath, password)
	if err != nil {
		t.Fatalf("failed to load vault: %v", err)
	}

	if len(loaded) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(loaded))
	}

	for i, entry := range loaded {
		if entry.Name != entries[i].Name || entry.Secret != entries[i].Secret ||
			entry.Algorithm != entries[i].Algorithm || entry.Digits != entries[i].Digits ||
			entry.Period != entries[i].Period {
			t.Errorf("entry mismatch at %d. Expected %+v, got %+v", i, entries[i], entry)
		}
	}

	// Try loading with wrong password
	_, err = LoadVault(vaultPath, "wrong-password")
	if err == nil {
		t.Fatal("expected decryption error with wrong password, but it succeeded")
	}
}
