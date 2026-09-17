package options

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const installIDEnvVar = "NUON_INSTALL_ID"

type RoleOptions struct {
	Disabled []string `json:"disabled"`
}

type RawCommonOptions struct {
	InstallID          string
	InputsPath         string
	SecretsPath        string
	Profile            string
	Watch              bool
	DisableRoles       []string
	DisableMaintenance bool
	DisableProvision   bool
	DisableDeprovision bool
}

type CommonOptions struct {
	InstallID   string
	InputsPath  string
	SecretsPath string
	Profile     string
	Watch       bool
	Roles       RoleOptions
	Inputs      map[string]any
	Secrets     map[string]any
}

func BindCommonFlags(cmd *cobra.Command, opts *RawCommonOptions) {
	cmd.Flags().StringVar(&opts.InstallID, "install-id", "", "Nuon install ID (falls back to NUON_INSTALL_ID)")
	cmd.Flags().StringVar(&opts.InputsPath, "inputs", "", "Path to required JSON object of non-secret CloudFormation parameters")
	cmd.Flags().StringVar(&opts.SecretsPath, "secrets", "", "Path to JSON object of secret-backed CloudFormation parameters (on stack updates, omitted or empty template secret values keep existing stack values)")
	cmd.Flags().StringVar(&opts.Profile, "profile", "", "AWS shared config profile used for CloudFormation and account verification (optional)")
	cmd.Flags().BoolVar(&opts.Watch, "watch", false, "Show live apply progress (spinner in TTY mode; plain text otherwise)")
	cmd.Flags().StringArrayVar(&opts.DisableRoles, "disable-role", nil, "Disable an assumable role by label, alias, or Enable* parameter name (repeatable)")
	cmd.Flags().BoolVar(&opts.DisableMaintenance, "disable-maintenance", false, "Alias for --disable-role maintenance")
	cmd.Flags().BoolVar(&opts.DisableProvision, "disable-provision", false, "Alias for --disable-role provision")
	cmd.Flags().BoolVar(&opts.DisableDeprovision, "disable-deprovision", false, "Alias for --disable-role deprovision")
}

func Normalize(raw RawCommonOptions, getenv func(string) string) (CommonOptions, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	installID := strings.TrimSpace(raw.InstallID)
	if installID == "" {
		installID = strings.TrimSpace(getenv(installIDEnvVar))
	}
	if installID == "" {
		return CommonOptions{}, fmt.Errorf("install ID is required: pass --install-id or set %s", installIDEnvVar)
	}

	inputsPath := strings.TrimSpace(raw.InputsPath)
	if inputsPath == "" {
		return CommonOptions{}, fmt.Errorf("inputs file path is required (--inputs)")
	}

	secretsPath := strings.TrimSpace(raw.SecretsPath)

	return CommonOptions{
		InstallID:   installID,
		InputsPath:  inputsPath,
		SecretsPath: secretsPath,
		Profile:     strings.TrimSpace(raw.Profile),
		Watch:       raw.Watch,
		Roles: RoleOptions{
			Disabled: normalizeDisabledRoles(raw),
		},
	}, nil
}

func normalizeDisabledRoles(raw RawCommonOptions) []string {
	names := append([]string{}, raw.DisableRoles...)
	if raw.DisableMaintenance {
		names = append(names, "maintenance")
	}
	if raw.DisableProvision {
		names = append(names, "provision")
	}
	if raw.DisableDeprovision {
		names = append(names, "deprovision")
	}

	seen := map[string]struct{}{}
	disabled := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		disabled = append(disabled, trimmed)
	}

	return disabled
}
