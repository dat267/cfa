package cmd

import (
	"fmt"
	"strings"
)

type RenameCmd struct {
	OldName string `arg:"" required:"" help:"Current account name"`
	NewName string `arg:"" required:"" help:"New account name"`
}

func (c *RenameCmd) Run(vaultPath VaultPath) error {
	oldName := c.OldName
	newName := strings.TrimSpace(c.NewName)
	if newName == "" {
		return fmt.Errorf("new account name cannot be empty")
	}

	password, err := getVaultPassword(string(vaultPath))
	if err != nil {
		return err
	}

	entries, err := LoadVault(string(vaultPath), password)
	if err != nil {
		return err
	}

	index := -1
	for i, entry := range entries {
		if strings.EqualFold(entry.Name, oldName) {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("no account found named '%s'", oldName)
	}

	for i, entry := range entries {
		if i != index && strings.EqualFold(entry.Name, newName) {
			return fmt.Errorf("an account named '%s' already exists", newName)
		}
	}

	actualOldName := entries[index].Name
	entries[index].Name = newName

	if err := SaveVault(string(vaultPath), entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully renamed '%s' to '%s'\033[0m\n", actualOldName, newName)
	return nil
}
