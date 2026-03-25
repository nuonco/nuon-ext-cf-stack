package runner

import (
	"strings"
	"testing"

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
		map[string]any{"foo": "value", "ignored": "skip"},
		map[string]any{"SecretA": "secret"},
		options.RoleOptions{Maintenance: true, Provision: false, Deprovision: true},
		map[string]struct{}{
			"ParameterFoo": {},
		},
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
		map[string]any{"shared": "input"},
		map[string]any{"ParameterShared": "secret"},
		options.RoleOptions{Maintenance: true, Provision: true, Deprovision: true},
		map[string]struct{}{
			"ParameterShared": {},
		},
	)
	if err == nil {
		t.Fatalf("expected duplicate parameter error")
	}
	if !strings.Contains(err.Error(), "duplicate parameter key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
