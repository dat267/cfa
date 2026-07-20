package cmd

import (
	"fmt"
	"os"
	"strings"
)

type InitCmd struct{}

func (c *InitCmd) Run(vaultPath VaultPath) error {
	p := string(vaultPath)
	if _, err := os.Stat(p); err == nil {
		fmt.Printf("\033[33mWarning: Vault already exists at %s.\033[0m\n", p)
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
	if err := SaveVault(p, emptyEntries, pwd); err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf("\033[32mSuccess: Vault securely initialized at %s\033[0m\n", p)
	return nil
}
