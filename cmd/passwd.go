package cmd

import (
	"flag"
	"fmt"
)

// PasswdCommand represents the change password command.
type PasswdCommand struct {
	fs        *flag.FlagSet
	vaultPath string
}

func NewPasswdCommand(vaultPath string) *PasswdCommand {
	return &PasswdCommand{
		fs:        flag.NewFlagSet("passwd", flag.ContinueOnError),
		vaultPath: vaultPath,
	}
}

func (c *PasswdCommand) Name() string        { return "passwd" }
func (c *PasswdCommand) Description() string { return "Change your master password" }
func (c *PasswdCommand) FlagSet() *flag.FlagSet { return c.fs }

func (c *PasswdCommand) Run(args []string) error {
	currentPwd, err := getVaultPassword(c.vaultPath)
	if err != nil {
		return err
	}

	entries, err := LoadVault(c.vaultPath, currentPwd)
	if err != nil {
		return err
	}

	newPwd, err := GetMasterPassword("Set new master password: ", true)
	if err != nil {
		return err
	}

	if currentPwd == newPwd {
		return fmt.Errorf("new password is identical to the current one")
	}

	if err := SaveVault(c.vaultPath, entries, newPwd); err != nil {
		return fmt.Errorf("failed to save vault with new password: %w", err)
	}

	fmt.Println("\033[32mSuccess: Master password successfully changed\033[0m")
	return nil
}
