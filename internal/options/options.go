package options

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const installIDEnvVar = "NUON_INSTALL_ID"

type RoleOptions struct {
	Maintenance bool `json:"maintenance"`
	Provision   bool `json:"provision"`
	Deprovision bool `json:"deprovision"`
}

type RawCommonOptions struct {
	InstallID          string
	InputsPath         string
	SecretsPath        string
	Profile            string
	Watch              bool
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
	cmd.Flags().BoolVar(&opts.DisableMaintenance, "disable-maintenance", false, "Set EnableRunnerMaintenance=false")
	cmd.Flags().BoolVar(&opts.DisableProvision, "disable-provision", false, "Set EnableRunnerProvision=false")
	cmd.Flags().BoolVar(&opts.DisableDeprovision, "disable-deprovision", false, "Set EnableRunnerDeprovision=false")
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

	roles := RoleOptions{
		Maintenance: !raw.DisableMaintenance,
		Provision:   !raw.DisableProvision,
		Deprovision: !raw.DisableDeprovision,
	}

	if !roles.Maintenance && !roles.Provision && !roles.Deprovision {
		return CommonOptions{}, fmt.Errorf("at least one role must remain enabled")
	}

	return CommonOptions{
		InstallID:   installID,
		InputsPath:  inputsPath,
		SecretsPath: secretsPath,
		Profile:     strings.TrimSpace(raw.Profile),
		Watch:       raw.Watch,
		Roles:       roles,
	}, nil
}
