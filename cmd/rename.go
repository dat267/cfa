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

	entries, password, err := vaultPath.Open()
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

	if err := vaultPath.Save(entries, password); err != nil {
		return err
	}

	fmt.Printf(colorGreen+"Successfully renamed '%s' to '%s'"+colorReset+"\n", actualOldName, newName)
	return nil
}
