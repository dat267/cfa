package main

import (
	"cfa/cmd"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	mincmd "github.com/dat267/min/cmd"
)

var version = "cfa/development"

func main() {
	app := &cmd.CLI{}
	parser, err := kong.New(app,
		kong.Name("cfa"),
		kong.Description("cfa - Cryptographically Secure CLI MFA Code Generator"),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	parser.FatalIfErrorf(err)

	if len(os.Args) < 2 {
		parser.Parse([]string{"--help"})
		return
	}

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	configFile := app.ConfigFile
	if configFile == "" {
		configFile = resolveConfigFile()
	}
	mincmd.SetConfigPath(configFile)

	vaultPath := resolveVaultPath()
	if data, err := os.ReadFile(configFile); err == nil {
		var cfg struct {
			VaultPath string `json:"vault-path"`
		}
		if json.Unmarshal(data, &cfg) == nil && cfg.VaultPath != "" {
			vaultPath = cfg.VaultPath
		}
	}

	ctx.Bind(cmd.VaultPath(vaultPath))
	ctx.BindTo(context.Background(), (*context.Context)(nil))

	if err := ctx.Run(); err != nil {
		if strings.Contains(err.Error(), "incorrect master password") {
			time.Sleep(2 * time.Second)
		}
		fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		os.Exit(1)
	}
}

func resolveConfigFile() string {
	if cf := os.Getenv("CFA_CONFIG_FILE"); cf != "" {
		return cf
	}
	localFile := "cfa.json"
	if _, err := os.Stat(localFile); err == nil {
		return localFile
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "cfa", "cfa.json")
	}
	return localFile
}

func resolveVaultPath() string {
	if p := os.Getenv("CFA_VAULT_PATH"); p != "" {
		return p
	}
	return cmd.DefaultVaultPath()
}
