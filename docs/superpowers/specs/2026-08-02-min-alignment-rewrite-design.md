# Design: Align cfa with min's structure

Date: 2026-08-02

## Goal

Rewrite `cfa` to follow the structural principles demonstrated by
`github.com/dat267/min`: a thin `main.go`, a `cmd` package exposing
`Execute(ctx)`, Kong-based config handling via a JSON resolver, version
injection via ldflags, and per-file unit tests.

All ten commands keep their behavior, flags, help text, colors, and error
handling. The only CLI surface change is a new `--vault-path` flag (with the
same effective precedence for existing users).

## Approach

Mirror min's file layout and conventions exactly, applying them to cfa's
domain.

## File structure

```
main.go               Thin entry point: version wiring + cmd.Execute(ctx)
version.go            package main: var version = "dev" (ldflags target)
cmd/root.go           NEW: appName, CLI struct, Version, Execute(ctx),
                      SetConfigPath/CfgPath/resolveConfigPath, JSONResolver,
                      resolveConfigFileFlag, VersionCmd. Absorbs main.go and
                      cmd/cli.go (deleted).
cmd/ui.go             NEW: color constants, confirmAction, getMasterPassword
                      (moved out of vault.go).
cmd/vault.go          VaultPath type, VaultEntry, loadVault/saveVault,
                      DefaultVaultPath (interactive helpers removed).
cmd/totp.go           Unchanged: QR decode, secret helpers, otpauth parse,
                      TOTP generation.
cmd/{add,init,list,show,remove,rename,passwd,export,import}.go
                      Unchanged: command structs with Run(vaultPath) error.
cmd/root_test.go      NEW: resolveConfigPath, resolveConfigFileFlag,
                      JSONResolver (flat/nested/malformed), VersionCmd.
cmd/totp_test.go      NEW: cleanSecret, validateBase32, parseOTPAuthURL,
                      parseAlgorithm, parseDigits, generateTOTP (RFC 6238).
cmd/vault_test.go     NEW: save/load round-trip, wrong-password error.
cmd/testutil_test.go  NEW: captureStdout helper (copied from min).
```

## Command/CLI surface

`CLI` struct in `cmd/root.go`:

```go
type CLI struct {
    ConfigFile string `help:"Config file path" json:"-"`
    VaultPath  string `help:"Vault file path" env:"CFA_VAULT_PATH" json:"-"`

    Init   InitCmd   `cmd:"" help:"Initialize the secure vault and set a master password"`
    Add    AddCmd    `cmd:"" help:"Add a new MFA token (from prompt, QR code image, or raw secret)"`
    List   ListCmd   `cmd:"" help:"Display the current and next TOTP codes for all accounts"`
    Show   ShowCmd   `cmd:"" help:"Show the current 6/8-digit code for a specific account"`
    Remove RemoveCmd `cmd:"" help:"Delete an account from the vault"`
    Rename RenameCmd `cmd:"" help:"Rename an account"`
    Passwd PasswdCmd `cmd:"" help:"Change your master password"`
    Export ExportCmd `cmd:"" help:"Export all entries as plain JSON (to stdout or file)"`
    Import ImportCmd `cmd:"" help:"Import entries from a plain JSON file"`
    Config mincmd.ConfigCmdGroup `cmd:"" help:"Manage application configuration"`
    Version VersionCmd `cmd:"" help:"Print version information"`
}
```

`Execute(ctx context.Context)` flow:

1. `resolveConfigFileFlag()` over `os.Args`; if found, `SetConfigPath(cf)`.
2. `activeConfig := CfgPath()`.
3. Build kong options: name, description, `UsageOnError`, compact help,
   `kong.BindTo(ctx, (*context.Context)(nil))`.
4. If config file opens, append `kong.Resolvers(JSONResolver(f))`.
5. `kong.New`, fail on error (stderr + exit 1).
6. If `len(os.Args) < 2`, parse `["--help"]` and return (preserves README's
   "no arguments shows help").
7. Parse args; fail on error.
8. `SetConfigPath(app.ConfigFile)` (min pattern; empty resets to resolved).
9. Resolve vault path: `app.VaultPath` if non-empty, else `DefaultVaultPath()`.
10. `kongCtx.Bind(VaultPath(vaultPath))`.
11. `kongCtx.Run()`; on error, sleep 2s if `errors.Is(err, ErrIncorrectPassword)`,
    print red error to stderr, exit 1.

## Config / vault path precedence

Config file resolved by `resolveConfigPath()`:
`CFA_CONFIG_FILE` env → local `./cfa.json` → `$XDG_CONFIG_HOME|UserConfigDir/cfa/cfa.json`.

The config file is loaded as a Kong JSON resolver (flat + nested keys, copied
from min's `JSONResolver`). The `vault-path` key therefore feeds the
`--vault-path` flag as a default.

Precedence (Kong): `--vault-path` flag > `CFA_VAULT_PATH` env > config
`vault-path` > `DefaultVaultPath()` fallback. This replaces the previous
manual config read in `main.go`; for users who only set the config key and no
env/flag, behavior is identical.

`SetConfigPath`/`CfgPath` keep a mutex-guarded package variable so the imported
`mincmd.ConfigCmdGroup` (`cfa config ...`) operates on cfa's config file.

## Version wiring

- `version.go` (package main): `var version = "dev"`.
- `cmd/root.go`: `var Version = "dev"`; `init()` reads
  `debug.ReadBuildInfo()` and overrides when a real module version exists.
- `main.go`: `if version != "dev" { cmd.Version = version }`.
- `VersionCmd.Run()` prints `cfa <Version>` (current format preserved).
- `release.yml` ldflags change from
  `-X github.com/dat267/cfa/cmd.Version=${{ env.BIN_NAME }}/${{ github.sha }}`
  to `-X main.version=${{ env.BIN_NAME }}/${{ github.sha }}`.

## Error handling

Unchanged behavior: `ErrIncorrectPassword` triggers a 2s sleep before the
error is printed (brute-force mitigation, README section 3); all run errors
print red to stderr with exit code 1. This logic moves from `main.go` into
`Execute`.

## Testing

- `cmd/root_test.go`: resolveConfigPath env + local-file variants,
  resolveConfigFileFlag table test, JSONResolver flat/nested/malformed,
  VersionCmd output.
- `cmd/totp_test.go`: secret cleaning, Base32 validation, otpauth URL parsing,
  algorithm/digits mapping, TOTP generation against an RFC 6238 test vector.
- `cmd/vault_test.go`: save then load round-trip via `t.TempDir()`, wrong
  password returns `ErrIncorrectPassword`.
- `cmd/testutil_test.go`: `captureStdout` helper.
- Verification: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`.
- `release.yml` also runs no lint job today; keep it that way unless build/test
  surface changes warrant one.

## Out of scope

- No changes to TOTP generation, vault crypto, QR decoding, or clipboard logic.
- No changes to README usage sections except nothing user-facing changes.
- No new commands.
