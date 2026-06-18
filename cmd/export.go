package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// ExportCommand represents the export vault entries command.
type ExportCommand struct {
	fs        *flag.FlagSet
	vaultPath string
	outOpt    string
}

func NewExportCommand(vaultPath string) *ExportCommand {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	c := &ExportCommand{fs: fs, vaultPath: vaultPath}
	fs.StringVar(&c.outOpt, "out", "", "Output JSON file path")
	return c
}

func (c *ExportCommand) Name() string        { return "export" }
func (c *ExportCommand) Description() string { return "Export all entries as plain JSON (to stdout or file)" }
func (c *ExportCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *ExportCommand) Run(args []string) error {
	password, err := getVaultPassword(c.vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(c.vaultPath, password)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize vault to JSON: %w", err)
	}

	if c.outOpt != "" {
		if err := os.WriteFile(c.outOpt, jsonData, 0600); err != nil {
			return fmt.Errorf("failed to write export file: %w", err)
		}
		fmt.Printf("\033[32mSuccessfully exported %d entries to %s\033[0m\n", len(entries), c.outOpt)
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}
