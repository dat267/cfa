package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

type ExportCmd struct {
	Out string `help:"Output file path" placeholder:"PATH"`
}

func (c *ExportCmd) Run(vaultPath VaultPath) error {
	password, err := getVaultPassword(string(vaultPath))
	if err != nil {
		return err
	}

	entries, err := LoadVault(string(vaultPath), password)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize vault to JSON: %w", err)
	}

	if c.Out != "" {
		if err := os.WriteFile(c.Out, jsonData, 0600); err != nil {
			return fmt.Errorf("failed to write export file: %w", err)
		}
		fmt.Printf("\033[32mSuccessfully exported %d entries to %s\033[0m\n", len(entries), c.Out)
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}
