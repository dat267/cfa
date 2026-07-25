package cmd

import (
	"fmt"

	mincmd "github.com/dat267/min/cmd"
)

type CLI struct {
	ConfigFile string `help:"Config file path"`

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
	Version VersionCmd   `cmd:"" help:"Print version information"`
}

type VersionCmd struct{}

func (c *VersionCmd) Run() error {
	fmt.Println("cfa cfa/development")
	return nil
}
