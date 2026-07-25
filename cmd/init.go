package cmd

import (
	"fmt"
	"os"
)

type InitCmd struct{}

func (c *InitCmd) Run(vaultPath VaultPath) error {
	p := string(vaultPath)
	if _, err := os.Stat(p); err == nil {
		ok, err := confirmAction(colorYellow+"Warning: Vault already exists at %s."+colorReset+"\nDo you want to re-initialize it? All current secrets will be lost!", p)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Aborted.")
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot check vault path %s: %w", p, err)
	}

	pwd, err := getMasterPassword("Set a master password: ", true)
	if err != nil {
		return err
	}

	if err := vaultPath.Save(nil, pwd); err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf(colorGreen+"Success: Vault securely initialized at %s"+colorReset+"\n", p)
	return nil
}
