package cmd

import (
	"fmt"
)

type PasswdCmd struct{}

func (c *PasswdCmd) Run(vaultPath VaultPath) error {
	entries, currentPwd, err := vaultPath.Open()
	if err != nil {
		return err
	}

	newPwd, err := getMasterPassword("Set new master password: ", true, false)
	if err != nil {
		return err
	}

	if currentPwd == newPwd {
		return fmt.Errorf("new password is identical to the current one")
	}

	if err := vaultPath.Save(entries, newPwd); err != nil {
		return fmt.Errorf("failed to save vault with new password: %w", err)
	}

	fmt.Println(colorGreen + "Success: Master password successfully changed" + colorReset)
	return nil
}
