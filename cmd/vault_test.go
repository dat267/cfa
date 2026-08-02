package cmd

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestVaultSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.enc")
	entries := []VaultEntry{
		{Name: "GitHub:john", Secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", Algorithm: "SHA1", Digits: 6, Period: 30},
		{Name: "Google:alice", Secret: "JBSWY3DPEHPK3PXP", Algorithm: "SHA256", Digits: 8, Period: 60},
	}

	if err := saveVault(path, entries, "test-password"); err != nil {
		t.Fatalf("saveVault: %v", err)
	}

	got, err := loadVault(path, "test-password")
	if err != nil {
		t.Fatalf("loadVault: %v", err)
	}

	if len(got) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(got), len(entries))
	}
	for i := range entries {
		if got[i] != entries[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], entries[i])
		}
	}
}

func TestVaultSaveCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "vault.enc")
	if err := saveVault(path, nil, "test-password"); err != nil {
		t.Fatalf("saveVault with missing parent dir: %v", err)
	}
	if _, err := loadVault(path, "test-password"); err != nil {
		t.Fatalf("loadVault: %v", err)
	}
}

func TestVaultWrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.enc")
	if err := saveVault(path, nil, "correct-password"); err != nil {
		t.Fatalf("saveVault: %v", err)
	}

	_, err := loadVault(path, "wrong-password")
	if !errors.Is(err, ErrIncorrectPassword) {
		t.Errorf("expected ErrIncorrectPassword, got %v", err)
	}
}

func TestVaultMissingFile(t *testing.T) {
	_, err := loadVault(filepath.Join(t.TempDir(), "nope.enc"), "password")
	if err == nil {
		t.Fatal("expected error for missing vault file, got nil")
	}
}
