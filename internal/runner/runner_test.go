package runner

import (
	"strings"
	"testing"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/nuonco/nuon/sdks/nuon-go/models"

	"github.com/nuonco/nuon-ext-cf-stack/internal/options"
)

func TestQuickCreateValue(t *testing.T) {
	quickLink := "https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/quickcreate?templateUrl=https%3A%2F%2Fexample.com%2Ftemplate.json&stackName=nuon-stack"

	if got := quickCreateValue(quickLink, "stackName"); got != "nuon-stack" {
		t.Fatalf("expected stackName nuon-stack, got %q", got)
	}

	if got := quickCreateValue(quickLink, "templateUrl"); got != "https://example.com/template.json" {
		t.Fatalf("expected decoded templateUrl, got %q", got)
	}
}

func TestStackConsoleURL(t *testing.T) {
	got := stackConsoleURL("us-east-1", "nuon-stack")
	want := "https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks?filteringStatus=active&filteringText=nuon-stack&hideStacks=false&viewNested=true"
	if got != want {
		t.Fatalf("expected stack console url %q, got %q", want, got)
	}

	if got := stackConsoleURL("", "nuon-stack"); got != "" {
		t.Fatalf("expected empty stack console url when region is empty, got %q", got)
	}
}

func TestLatestInstallStackVersion(t *testing.T) {
	versions := []*models.AppInstallStackVersion{
		{ID: "old", CreatedAt: "2025-01-01T00:00:00Z"},
		{ID: "new", CreatedAt: "2026-01-01T00:00:00Z"},
	}

	latest := latestInstallStackVersion(versions)
	if latest == nil || latest.ID != "new" {
		t.Fatalf("expected newest version, got %#v", latest)
	}
}

func TestBuildStackParametersIncludesRoleFlags(t *testing.T) {
	params, err := buildStackParameters(
		"install",
		map[string]any{"foo": "value", "ignored": "skip"},
		map[string]any{"SecretA": "secret"},
		options.RoleOptions{Maintenance: true, Provision: false, Deprovision: true},
		map[string]struct{}{
			"ParameterFoo": {},
			"SecretA":      {},
		},
		map[string]struct{}{
			"SecretA": {},
		},
		nil,
		false,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	joined := ""
	for _, p := range params {
		joined += *p.ParameterKey + "=" + *p.ParameterValue + ";"
	}

	for _, expected := range []string{
		enableRunnerMaintenanceParam + "=true",
		enableRunnerProvisionParam + "=false",
		enableRunnerDeprovisionParam + "=true",
		"ParameterFoo=value",
		"SecretA=secret",
	} {
		if !strings.Contains(joined, expected+";") {
			t.Fatalf("expected %q in parameters: %s", expected, joined)
		}
	}

	if strings.Contains(joined, "ignored=") {
		t.Fatalf("expected ignored input key to be omitted, got %s", joined)
	}
}

func TestBuildStackParametersRejectsDuplicateKeys(t *testing.T) {
	_, err := buildStackParameters(
		"install",
		map[string]any{"shared": "input"},
		map[string]any{"ParameterShared": "secret"},
		options.RoleOptions{Maintenance: true, Provision: true, Deprovision: true},
		map[string]struct{}{
			"ParameterShared": {},
		},
		nil,
		nil,
		false,
	)
	if err == nil {
		t.Fatalf("expected duplicate parameter error")
	}
	if !strings.Contains(err.Error(), "duplicate parameter key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStackParametersUpgradeKeepsOmittedSecretValues(t *testing.T) {
	params, err := buildStackParameters(
		"upgrade",
		map[string]any{"foo": "value"},
		map[string]any{},
		options.RoleOptions{Maintenance: true, Provision: true, Deprovision: true},
		map[string]struct{}{
			"ParameterFoo":      {},
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		true,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parameterByKey := parametersByKey(params)
	secret, ok := parameterByKey["GithubAppKeyParam"]
	if !ok {
		t.Fatalf("expected GithubAppKeyParam to be included")
	}
	if secret.UsePreviousValue == nil || !*secret.UsePreviousValue {
		t.Fatalf("expected GithubAppKeyParam to use previous value, got %#v", secret)
	}
	if secret.ParameterValue != nil {
		t.Fatalf("expected no direct parameter value for GithubAppKeyParam, got %#v", secret.ParameterValue)
	}
}

func TestBuildStackParametersUpgradeUsesProvidedSecretValues(t *testing.T) {
	params, err := buildStackParameters(
		"upgrade",
		map[string]any{"foo": "value"},
		map[string]any{"GithubAppKeyParam": "updated-secret"},
		options.RoleOptions{Maintenance: true, Provision: true, Deprovision: true},
		map[string]struct{}{
			"ParameterFoo":      {},
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		true,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parameterByKey := parametersByKey(params)
	secret, ok := parameterByKey["GithubAppKeyParam"]
	if !ok {
		t.Fatalf("expected GithubAppKeyParam to be included")
	}
	if secret.ParameterValue == nil || *secret.ParameterValue != "updated-secret" {
		t.Fatalf("expected provided secret value, got %#v", secret.ParameterValue)
	}
	if secret.UsePreviousValue != nil && *secret.UsePreviousValue {
		t.Fatalf("expected provided secret value to override use-previous behavior")
	}
}

func TestBuildStackParametersInstallKeepsOmittedSecretValuesWhenStackExists(t *testing.T) {
	params, err := buildStackParameters(
		"install",
		map[string]any{"foo": "value"},
		map[string]any{},
		options.RoleOptions{Maintenance: true, Provision: true, Deprovision: true},
		map[string]struct{}{
			"ParameterFoo":      {},
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		true,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parameterByKey := parametersByKey(params)
	secret, ok := parameterByKey["GithubAppKeyParam"]
	if !ok {
		t.Fatalf("expected GithubAppKeyParam to be included")
	}
	if secret.UsePreviousValue == nil || !*secret.UsePreviousValue {
		t.Fatalf("expected GithubAppKeyParam to use previous value, got %#v", secret)
	}
}

func TestBuildStackParametersInstallTreatsEmptySecretAsKeepExisting(t *testing.T) {
	params, err := buildStackParameters(
		"install",
		map[string]any{"foo": "value"},
		map[string]any{"GithubAppKeyParam": ""},
		options.RoleOptions{Maintenance: true, Provision: true, Deprovision: true},
		map[string]struct{}{
			"ParameterFoo":      {},
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		map[string]struct{}{
			"GithubAppKeyParam": {},
		},
		true,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parameterByKey := parametersByKey(params)
	secret, ok := parameterByKey["GithubAppKeyParam"]
	if !ok {
		t.Fatalf("expected GithubAppKeyParam to be included")
	}
	if secret.UsePreviousValue == nil || !*secret.UsePreviousValue {
		t.Fatalf("expected GithubAppKeyParam to use previous value when empty secret was provided, got %#v", secret)
	}
	if secret.ParameterValue != nil {
		t.Fatalf("expected no direct parameter value when empty secret was provided, got %#v", secret.ParameterValue)
	}
}

func parametersByKey(params []cftypes.Parameter) map[string]cftypes.Parameter {
	indexed := make(map[string]cftypes.Parameter, len(params))
	for _, parameter := range params {
		if parameter.ParameterKey == nil {
			continue
		}
		indexed[*parameter.ParameterKey] = parameter
	}

	return indexed
}
