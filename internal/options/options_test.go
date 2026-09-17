package options

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeDefaults(t *testing.T) {
	raw := RawCommonOptions{
		InputsPath:  "inputs.json",
		SecretsPath: "secrets.json",
	}

	opts, err := Normalize(raw, func(key string) string {
		if key == "NUON_INSTALL_ID" {
			return "inl_env"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if opts.InstallID != "inl_env" {
		t.Fatalf("expected install ID from env, got %q", opts.InstallID)
	}

	if len(opts.Roles.Disabled) != 0 {
		t.Fatalf("expected no roles disabled by default, got %+v", opts.Roles.Disabled)
	}
}

func TestNormalizeInstallIDFlagOverridesEnv(t *testing.T) {
	raw := RawCommonOptions{
		InstallID:   "inl_flag",
		InputsPath:  "inputs.json",
		SecretsPath: "secrets.json",
		Profile:     " default ",
		Watch:       true,
	}

	opts, err := Normalize(raw, func(string) string { return "inl_env" })
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if opts.InstallID != "inl_flag" {
		t.Fatalf("expected install ID from flag, got %q", opts.InstallID)
	}

	if opts.Profile != "default" {
		t.Fatalf("expected trimmed profile, got %q", opts.Profile)
	}

	if !opts.Watch {
		t.Fatalf("expected watch to be enabled")
	}
}

func TestNormalizeRoleToggles(t *testing.T) {
	raw := RawCommonOptions{
		InstallID:          "inl_123",
		InputsPath:         "inputs.json",
		SecretsPath:        "secrets.json",
		DisableMaintenance: true,
		DisableRoles:       []string{" dynamodb-operations ", "deprovision", "dynamodb-operations"},
	}

	opts, err := Normalize(raw, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{"dynamodb-operations", "deprovision", "maintenance"}
	if !reflect.DeepEqual(opts.Roles.Disabled, want) {
		t.Fatalf("expected disabled roles %v, got %v", want, opts.Roles.Disabled)
	}
}

func TestNormalizeAllowsAllFirstClassRolesDisabled(t *testing.T) {
	raw := RawCommonOptions{
		InstallID:          "inl_123",
		InputsPath:         "inputs.json",
		SecretsPath:        "secrets.json",
		DisableMaintenance: true,
		DisableProvision:   true,
		DisableDeprovision: true,
	}

	opts, err := Normalize(raw, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	want := []string{"maintenance", "provision", "deprovision"}
	if !reflect.DeepEqual(opts.Roles.Disabled, want) {
		t.Fatalf("expected disabled roles %v, got %v", want, opts.Roles.Disabled)
	}
}

func TestNormalizeRequiresInstallID(t *testing.T) {
	raw := RawCommonOptions{
		InputsPath: "inputs.json",
	}

	_, err := Normalize(raw, nil)
	if err == nil {
		t.Fatalf("expected install ID validation error")
	}
	if !strings.Contains(err.Error(), "install ID is required") {
		t.Fatalf("expected install ID error, got %v", err)
	}
}

func TestNormalizeAllowsOmittedSecretsPath(t *testing.T) {
	raw := RawCommonOptions{
		InstallID:  "inl_123",
		InputsPath: "inputs.json",
	}

	opts, err := Normalize(raw, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if opts.SecretsPath != "" {
		t.Fatalf("expected empty secrets path, got %q", opts.SecretsPath)
	}
}
