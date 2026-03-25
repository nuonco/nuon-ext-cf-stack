package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nuonco/nuon-ext-cf-stack/internal/config"
	"github.com/nuonco/nuon-ext-cf-stack/internal/options"
	"github.com/nuonco/nuon-ext-cf-stack/internal/runner"
)

func newInstallCmd() *cobra.Command {
	raw := options.RawCommonOptions{}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install CF stack resources for an install",
		Long:  "Install CloudFormation stack resources for a Nuon install. Requires --inputs JSON. On first create, provide required secret-backed parameters via --secrets. When install targets an existing stack, omitted or empty template secret values keep existing stack values.",
		Example: "  nuon cf-stack install --install-id inl_123 --inputs inputs.json\n" +
			"  nuon cf-stack install --install-id inl_123 --inputs inputs.json --secrets secrets.json\n" +
			"  NUON_DEBUG=true nuon cf-stack install --install-id inl_123 --inputs inputs.json --disable-deprovision --watch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := options.Normalize(raw, os.Getenv)
			if err != nil {
				return err
			}

			opts.Inputs, err = config.LoadJSONObject(opts.InputsPath)
			if err != nil {
				return fmt.Errorf("failed to load inputs: %w", err)
			}

			opts.Secrets = map[string]any{}
			if opts.SecretsPath != "" {
				opts.Secrets, err = config.LoadJSONObject(opts.SecretsPath)
				if err != nil {
					return fmt.Errorf("failed to load secrets: %w", err)
				}
			}

			return runner.Run(cmd.Context(), cmd.OutOrStdout(), "install", opts)
		},
	}

	options.BindCommonFlags(cmd, &raw)

	return cmd
}
