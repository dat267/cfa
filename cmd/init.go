package cmd

import (
	"fmt"
)

type InitCmd struct{}

func (c *InitCmd) Run(vaultPath VaultPath) error {
	exists, err := vaultPath.Exists()
	if err != nil {
		return fmt.Errorf("cannot check vault path: %w", err)
	}
	if exists {
		ok, err := confirmAction(colorYellow+"Warning: Vault already exists at %s."+colorReset+"\nDo you want to re-initialize it? All current secrets will be lost!", string(vaultPath))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Aborted.")
			return nil
		}
	}

	pwd, err := getMasterPassword("Set a master password: ", true, false)
	if err != nil {
		return err
	}

	if err := vaultPath.Save(nil, pwd); err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf(colorGreen+"Success: Vault securely initialized at %s"+colorReset+"\n", string(vaultPath))
	return nil
}
