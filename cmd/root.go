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
		Use:          "cf-stack",
		Short:        "Manage CF stack install and upgrade workflows",
		Long:         "Manage CloudFormation install and upgrade workflows for a Nuon install. Both commands require --inputs (a JSON object map of CloudFormation parameters). Use --secrets for secret-backed parameters.",
		Example:      "  nuon cf-stack upgrade --install-id inl_123 --inputs inputs.json\n  nuon cf-stack upgrade --install-id inl_123 --inputs inputs.json --secrets secrets.json --disable-deprovision\n  NUON_DEBUG=true nuon cf-stack install --inputs inputs.json --watch",
		SilenceUsage: true,
	}
	root.Version = BuildVersion

	root.AddCommand(newInstallCmd())
	root.AddCommand(newUpgradeCmd())

	return root
}
