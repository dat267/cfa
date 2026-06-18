package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// RemoveCommand represents the command to delete a credential.
type RemoveCommand struct {
	fs        *flag.FlagSet
	vaultPath string
}

func NewRemoveCommand(vaultPath string) *RemoveCommand {
	return &RemoveCommand{
		fs:        flag.NewFlagSet("remove", flag.ContinueOnError),
		vaultPath: vaultPath,
	}
}

func (c *RemoveCommand) Name() string           { return "remove" }
func (c *RemoveCommand) Description() string    { return "Delete an account from the vault" }
func (c *RemoveCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *RemoveCommand) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing account name. Usage: cfa remove <name>")
	}
	query := args[0]

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
		if strings.EqualFold(entry.Name, query) {
			index = i
			break
		}
	}

	if index == -1 {
		// Try substring match
		var matches []int
		for i, entry := range entries {
			if strings.Contains(strings.ToLower(entry.Name), strings.ToLower(query)) {
				matches = append(matches, i)
			}
		}
		if len(matches) == 0 {
			return fmt.Errorf("no accounts match '%s'", query)
		}
		if len(matches) > 1 {
			fmt.Fprintln(os.Stderr, "Multiple matches found:")
			for _, idx := range matches {
				fmt.Fprintf(os.Stderr, "  - %s\n", entries[idx].Name)
			}
			return fmt.Errorf("ambiguous query '%s', please be more specific", query)
		}
		index = matches[0]
	}

	targetName := entries[index].Name
	fmt.Printf("Are you sure you want to permanently delete account '%s'? [y/N]: ", targetName)
	var resp string
	fmt.Scanln(&resp)
	resp = strings.ToLower(strings.TrimSpace(resp))
	if resp != "y" && resp != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	// Remove element from slice
	entries = append(entries[:index], entries[index+1:]...)

	if err := SaveVault(c.vaultPath, entries, password); err != nil {
		return err
	}

	fmt.Printf("\033[32mSuccessfully deleted account '%s'\033[0m\n", targetName)
	return nil
}
