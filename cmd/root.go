package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
)

const appName = "cfa"

var Version = "dev"

var (
	cfgPathMu sync.RWMutex
	cfgPath   string
)

func init() {
	cfgPath = resolveConfigPath()
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}

// CLI is the root CLI struct containing all subcommand groups.
type CLI struct {
	ConfigFile string `help:"Config file path" json:"-"`
	VaultPath  string `help:"Vault file path" env:"CFA_VAULT_PATH"`

	Init   InitCmd   `cmd:"" help:"Initialize the secure vault and set a master password"`
	Add    AddCmd    `cmd:"" help:"Add a new MFA token (from prompt, QR code image, or raw secret)"`
	List   ListCmd   `cmd:"" help:"Display the current and next TOTP codes for all accounts"`
	Show   ShowCmd   `cmd:"" help:"Show the current 6/8-digit code for a specific account"`
	Remove RemoveCmd `cmd:"" help:"Delete an account from the vault"`
	Rename RenameCmd `cmd:"" help:"Rename an account"`
	Passwd PasswdCmd `cmd:"" help:"Change your master password"`
	Export ExportCmd `cmd:"" help:"Export all entries as plain JSON (to stdout or file)"`
	Import ImportCmd `cmd:"" help:"Import entries from a plain JSON file"`
	Config ConfigCmdGroup `cmd:"" help:"Manage application configuration"`
	Version VersionCmd `cmd:"" help:"Print version information"`
}

// SetConfigPath overrides the config file path used by config commands.
func SetConfigPath(p string) {
	cfgPathMu.Lock()
	defer cfgPathMu.Unlock()
	if p == "" {
		cfgPath = resolveConfigPath()
	} else {
		cfgPath = p
	}
}

func CfgPath() string {
	cfgPathMu.RLock()
	defer cfgPathMu.RUnlock()
	return cfgPath
}

func resolveConfigPath() string {
	envKey := strings.ToUpper(appName) + "_CONFIG_FILE"
	if cf := os.Getenv(envKey); cf != "" {
		return cf
	}
	localFile := appName + ".json"
	if _, err := os.Stat(localFile); err == nil {
		return localFile
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, appName, appName+".json")
	}
	return localFile
}

// Execute is the main entry point called by main.go.
func Execute(ctx context.Context) {
	app := &CLI{}

	if cf := resolveConfigFileFlag(); cf != "" {
		SetConfigPath(cf)
	}
	activeConfig := CfgPath()

	options := []kong.Option{
		kong.Name(appName),
		kong.Description("cfa - Cryptographically Secure CLI MFA Code Generator"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.BindTo(ctx, (*context.Context)(nil)),
	}

	if f, err := os.Open(activeConfig); err == nil {
		if resolver, err := JSONResolver(f); err == nil {
			options = append(options, kong.Resolvers(resolver))
		}
		_ = f.Close()
	}

	k, err := kong.New(app, options...)
	if err != nil {
		fmt.Fprintf(os.Stderr, colorRed+"Error: %v"+colorReset+"\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		_, err := k.Parse([]string{"--help"})
		k.FatalIfErrorf(err)
		return
	}

	kongCtx, err := k.Parse(os.Args[1:])
	k.FatalIfErrorf(err)

	SetConfigPath(app.ConfigFile)

	vaultPath := app.VaultPath
	if vaultPath == "" {
		vaultPath = DefaultVaultPath()
	}
	kongCtx.Bind(VaultPath(vaultPath))

	if err := kongCtx.Run(); err != nil {
		if errors.Is(err, ErrIncorrectPassword) {
			time.Sleep(2 * time.Second)
		}
		fmt.Fprintf(os.Stderr, colorRed+"Error: %v"+colorReset+"\n", err)
		os.Exit(1)
	}
}

func resolveConfigFileFlag() string {
	for i, arg := range os.Args {
		if arg == "--config-file" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(arg, "--config-file=") {
			parts := strings.SplitN(arg, "=", 2)
			return parts[1]
		}
	}
	return ""
}

// JSONResolver builds a Kong resolver capable of loading both flat and nested JSON configuration.
func JSONResolver(r io.Reader) (kong.Resolver, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	flat := make(map[string]any)

	var flattenNested func(prefix string, m map[string]any)
	flattenNested = func(prefix string, m map[string]any) {
		for k, v := range m {
			key := k
			if prefix != "" {
				key = prefix + "-" + k
			}
			if sub, ok := v.(map[string]any); ok {
				flattenNested(key, sub)
			} else if prefix != "" {
				flat[key] = v
			}
		}
	}
	flattenNested("", raw)

	for k, v := range raw {
		if _, isMap := v.(map[string]any); !isMap {
			flat[k] = v
		}
	}

	return kong.ResolverFunc(func(ctx *kong.Context, parent *kong.Path, flag *kong.Flag) (any, error) {
		if val, ok := flat[flag.Name]; ok {
			return val, nil
		}
		return nil, nil
	}), nil
}

type VersionCmd struct{}

func (c *VersionCmd) Run() error {
	fmt.Printf("cfa %s\n", Version)
	return nil
}
