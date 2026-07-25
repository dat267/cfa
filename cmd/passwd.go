package cmd

import (
	"fmt"
)

type PasswdCmd struct{}

func (c *PasswdCmd) Run(vaultPath VaultPath) error {
	p := string(vaultPath)
	currentPwd, err := getVaultPassword(p)
	if err != nil {
		return err
	}

	entries, err := LoadVault(p, currentPwd)
	if err != nil {
		return err
	}

	newPwd, err := getMasterPassword("Set new master password: ", true)
	if err != nil {
		return err
	}

	if currentPwd == newPwd {
		return fmt.Errorf("new password is identical to the current one")
	}

	if err := SaveVault(p, entries, newPwd); err != nil {
		return fmt.Errorf("failed to save vault with new password: %w", err)
	}

	fmt.Println(colorGreen + "Success: Master password successfully changed" + colorReset)
	return nil
}
