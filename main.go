package main

import (
	"cfa/cmd"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
)

var version = "cfa/development"

type Config struct {
	VaultPath string `json:"vault-path"`
}

type CLI struct {
	ConfigFile string `help:"Path to config file." placeholder:"PATH"`

	Init   cmd.InitCmd   `cmd:"" help:"Initialize the secure vault and set a master password"`
	Add    cmd.AddCmd    `cmd:"" help:"Add a new MFA token (from prompt, QR code image, or raw secret)"`
	List   cmd.ListCmd   `cmd:"" help:"Display the current and next TOTP codes for all accounts"`
	Show   cmd.ShowCmd   `cmd:"" help:"Show the current 6/8-digit code for a specific account"`
	Remove cmd.RemoveCmd `cmd:"" help:"Delete an account from the vault"`
	Rename cmd.RenameCmd `cmd:"" help:"Rename an account"`
	Passwd cmd.PasswdCmd `cmd:"" help:"Change your master password"`
	Export cmd.ExportCmd `cmd:"" help:"Export all entries as plain JSON (to stdout or file)"`
	Import cmd.ImportCmd `cmd:"" help:"Import entries from a plain JSON file"`
	Config ConfigCmdGroup `cmd:"" help:"Manage application configuration"`
	Version VersionCmd   `cmd:"" help:"Print version information"`
}

type VersionCmd struct{}

func (c *VersionCmd) Run() error {
	fmt.Printf("cfa %s\n", version)
	return nil
}

type ConfigCmdGroup struct {
	Init ConfigInitCmd `cmd:"" help:"Generate a default configuration template file"`
	Path ConfigPathCmd `cmd:"" help:"Show the active configuration file path"`
}

type ConfigInitCmd struct {
	Force bool `short:"f" help:"Overwrite existing configuration file"`
}

type ConfigPathCmd struct{}

func (c *ConfigInitCmd) Run(path cmd.ConfigPath) error {
	p := string(path)
	if _, err := os.Stat(p); err == nil && !c.Force {
		return fmt.Errorf("configuration file already exists at %s", p)
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create configuration directory: %w", err)
	}
	cfg := Config{}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}
	fmt.Printf("Successfully generated base configuration file at: %s\n", p)
	return nil
}

func (c *ConfigPathCmd) Run(path cmd.ConfigPath) error {
	p := string(path)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		fmt.Printf("%s (does not exist)\n", p)
		return nil
	}
	fmt.Println(p)
	return nil
}

func main() {
	resolveAppName := func() string {
		name := filepath.Base(os.Args[0])
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if name == "" || name == "main" || name == "app" ||
			strings.HasPrefix(name, "go-build") || strings.HasSuffix(name, ".test") {
			return "cfa"
		}
		return name
	}

	resolveConfigFile := func(appName string) string {
		for i, arg := range os.Args {
			if arg == "--config-file" && i+1 < len(os.Args) {
				return os.Args[i+1]
			}
			if after, found := strings.CutPrefix(arg, "--config-file="); found {
				return after
			}
		}
		envKey := strings.ToUpper(appName) + "_CONFIG_FILE"
		if configFile := os.Getenv(envKey); configFile != "" {
			return configFile
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

	appName := resolveAppName()
	configFile := resolveConfigFile(appName)

	kebabCase := func(s string) string {
		var sb strings.Builder
		sb.Grow(len(s) + 4)
		for i, r := range s {
			if i > 0 && r >= 'A' && r <= 'Z' {
				sb.WriteRune('-')
			}
			if r >= 'A' && r <= 'Z' {
				sb.WriteRune(r + ('a' - 'A'))
			} else {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}

	setFieldValue := func(fv reflect.Value, s string) {
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(s)
		case reflect.Bool:
			fv.SetBool(s == "true" || s == "1")
		case reflect.Int, reflect.Int64:
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				fv.SetInt(n)
			}
		}
	}

	type configField struct {
		value      reflect.Value
		defaultTag string
	}

	configFields := make(map[string]configField)
	var buildFlatMap func(reflect.Value, string)
	buildFlatMap = func(val reflect.Value, prefix string) {
		val = reflect.Indirect(val)
		if val.Kind() != reflect.Struct {
			return
		}
		t := val.Type()
		for i := 0; i < val.NumField(); i++ {
			fv := val.Field(i)
			ft := t.Field(i)

			name := kebabCase(ft.Name)
			if jsonTag := ft.Tag.Get("json"); jsonTag != "" {
				if parts := strings.Split(jsonTag, ","); parts[0] != "" && parts[0] != "-" {
					name = parts[0]
				}
			}

			fullKey := name
			if prefix != "" {
				fullKey = prefix + "-" + name
			}

			if fv.Kind() == reflect.Struct {
				buildFlatMap(fv, fullKey)
			} else {
				configFields[fullKey] = configField{value: fv, defaultTag: ft.Tag.Get("default")}
			}
		}
	}

	runtimeCfg := &Config{}
	explicitlySet := make(map[string]bool)

	buildFlatMap(reflect.ValueOf(runtimeCfg), "")

	var rawMap map[string]any
	if data, err := os.ReadFile(filepath.Clean(configFile)); err == nil {
		_ = json.Unmarshal(data, runtimeCfg)
		_ = json.Unmarshal(data, &rawMap)
	}

	explicitlySetPaths := make(map[string]string)

	var markExplicit func(map[string]any, string, string) error
	markExplicit = func(m map[string]any, flatPrefix string, dotPrefix string) error {
		for k, v := range m {
			flatKey := k
			dotKey := k
			if flatPrefix != "" {
				flatKey = flatPrefix + "-" + k
				dotKey = dotPrefix + "." + k
			}
			if sub, ok := v.(map[string]any); ok {
				if err := markExplicit(sub, flatKey, dotKey); err != nil {
					return err
				}
			} else {
				if _, ok := configFields[flatKey]; ok {
					if v != nil {
						if prev, exists := explicitlySetPaths[flatKey]; exists {
							return fmt.Errorf("both %q and %q are defined", prev, dotKey)
						}
						explicitlySetPaths[flatKey] = dotKey
						explicitlySet[flatKey] = true
					}
				}
			}
		}
		return nil
	}

	configResolver := kong.ResolverFunc(func(ctx *kong.Context, parent *kong.Path, flag *kong.Flag) (any, error) {
		if field, ok := configFields[flag.Name]; ok {
			fv := field.value
			if !explicitlySet[flag.Name] && flag.HasDefault {
				return nil, nil
			}
			if fv.IsZero() {
				return nil, nil
			}
			return fmt.Sprintf("%v", fv.Interface()), nil
		}
		return nil, nil
	})

	cli := &CLI{}
	k, err := kong.New(cli,
		kong.Name(appName),
		kong.Description("cfa - Cryptographically Secure CLI MFA Code Generator"),
		kong.UsageOnError(),
		kong.DefaultEnvars(strings.ToUpper(appName)),
		kong.Resolvers(configResolver),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Tree:    true,
		}),
	)
	k.FatalIfErrorf(err)

	if len(os.Args) < 2 {
		k.Parse([]string{"--help"})
		return
	}

	isConfigCmd := false
	if traceCtx, err2 := kong.Trace(k, os.Args[1:]); err2 == nil {
		isConfigCmd = strings.HasPrefix(traceCtx.Command(), "config")
	}

	if err = markExplicit(rawMap, "", ""); err != nil && !isConfigCmd {
		fmt.Fprintf(os.Stderr, "error: duplicate config keys in %s: %v. Run '%s config edit' to fix this.\n", configFile, err, appName)
		os.Exit(1)
	}

	envPrefix := strings.ToUpper(appName) + "_"
	for key, field := range configFields {
		envKey := envPrefix + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		if val, ok := os.LookupEnv(envKey); ok {
			setFieldValue(field.value, val)
			explicitlySet[key] = true
		} else if field.defaultTag != "" && !explicitlySet[key] && field.value.IsZero() {
			setFieldValue(field.value, field.defaultTag)
		}
	}

	vaultPath, err := cmd.DefaultVaultPath()
	if err != nil {
		vaultPath = filepath.Join(".", "vault.enc")
	}
	if runtimeCfg.VaultPath != "" {
		vaultPath = runtimeCfg.VaultPath
	}

	ctx, err := k.Parse(os.Args[1:])
	k.FatalIfErrorf(err)

	for _, flag := range ctx.Flags() {
		if field, ok := configFields[flag.Name]; ok {
			field.value.Set(flag.Target)
		}
	}

	ctx.Bind(runtimeCfg)
	ctx.Bind(cmd.ConfigPath(configFile))
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
