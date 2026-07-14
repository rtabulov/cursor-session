package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

const upgradeUnsupportedMsg = "upgrade is not supported on this fork yet"

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade cursor-session (" + upgradeUnsupportedMsg + ")",
		Long: upgradeUnsupportedMsg + `.

Binary releases and in-place upgrades will come later for
github.com/rtabulov/cursor-session. Until then, install or update
from source with go install or by building this repository.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New(upgradeUnsupportedMsg)
		},
	}
}

var upgradeCmd = newUpgradeCmd()

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
