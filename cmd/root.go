package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// BuildVersion is set at build time through ldflags.
var BuildVersion = "dev"

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "nuon-ext-cf-stack",
		Short:        "Manage CF stack install and upgrade workflows",
		SilenceUsage: true,
	}
	root.Version = BuildVersion

	root.AddCommand(newInstallCmd())
	root.AddCommand(newUpgradeCmd())

	return root
}
