package cmd

import (
	"flag"
	"fmt"
	"strings"
)

// RenameCommand represents the command to rename a credential.
type RenameCommand struct {
	fs        *flag.FlagSet
	vaultPath string
}

func NewRenameCommand(vaultPath string) *RenameCommand {
	return &RenameCommand{
		fs:        flag.NewFlagSet("rename", flag.ContinueOnError),
		vaultPath: vaultPath,
	}
}

func (c *RenameCommand) Name() string           { return "rename" }
func (c *RenameCommand) Description() string    { return "Rename an account" }
func (c *RenameCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *RenameCommand) Run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("missing arguments. Usage: cfa rename <old_name> <new_name>")
	}
	oldName := args[0]
	newName := strings.TrimSpace(args[1])
	if newName == "" {
		return fmt.Errorf("new account name cannot be empty")
	}

	password, err := getVaultPassword(c.vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(c.vaultPath, password)
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

	// Check if target name already exists
	for i, entry := range entries {
		if i != index && strings.EqualFold(entry.Name, newName) {
			return fmt.Errorf("an account named '%s' already exists", newName)
		}
	}

	actualOldName := entries[index].Name
	entries[index].Name = newName

	if err := SaveVault(c.vaultPath, entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully renamed '%s' to '%s'\033[0m\n", actualOldName, newName)
	return nil
}
