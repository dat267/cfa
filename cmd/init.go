package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// InitCommand represents the vault initialization command.
type InitCommand struct {
	fs *flag.FlagSet
}

func NewInitCommand() *InitCommand {
	return &InitCommand{
		fs: flag.NewFlagSet("init", flag.ContinueOnError),
	}
}

func (c *InitCommand) Name() string        { return "init" }
func (c *InitCommand) Description() string { return "Initialize the secure vault and set a master password" }
func (c *InitCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *InitCommand) Run(args []string) error {
	if _, err := os.Stat(VaultPath); err == nil {
		fmt.Printf("\033[33mWarning: Vault already exists at %s.\033[0m\n", VaultPath)
		fmt.Print("Do you want to re-initialize it? All current secrets will be lost! [y/N]: ")
		var resp string
		fmt.Scanln(&resp)
		resp = strings.ToLower(strings.TrimSpace(resp))
		if resp != "y" && resp != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	pwd, err := GetMasterPassword("Set a master password: ", true)
	if err != nil {
		return err
	}

	var emptyEntries []VaultEntry
	if err := SaveVault(VaultPath, emptyEntries, pwd); err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf("\033[32mSuccess: Vault securely initialized at %s\033[0m\n", VaultPath)
	return nil
}
