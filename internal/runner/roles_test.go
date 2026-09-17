package runner

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleAssumableTemplate = `{
  "Parameters": {
    "EnableRunnerMaintenance": {"Type": "String"},
    "EnableRunnerProvision": {"Type": "String"},
    "EnableRunnerDeprovision": {"Type": "String"},
    "EnableCustomNuonInstallIdDynamodbOperations": {"Type": "String"},
    "EnableCustomNuonInstallIdIngressOperations": {"Type": "String"},
    "VpcCIDR": {"Type": "String"}
  },
  "Metadata": {
    "AWS::CloudFormation::Interface": {
      "ParameterLabels": {
        "EnableRunnerMaintenance": "Maintenance",
        "EnableRunnerProvision": {"default": "Provision"},
        "EnableRunnerDeprovision": "Deprovision",
        "EnableCustomNuonInstallIdDynamodbOperations": "dynamodb-operations",
        "EnableCustomNuonInstallIdIngressOperations": "ingress-operations"
      }
    }
  },
  "Resources": {
    "RunnerMaintenance": {
      "Type": "AWS::IAM::Role",
      "Condition": "EnableRunnerMaintenance",
      "Properties": {
        "Tags": [{"Key": "runner.nuon.co/assumable", "Value": "true"}]
      }
    },
    "RunnerProvision": {
      "Type": "AWS::IAM::Role",
      "Condition": "EnableRunnerProvision",
      "Properties": {
        "Tags": [{"Key": "runner.nuon.co/assumable", "Value": "true"}]
      }
    },
    "RunnerDeprovision": {
      "Type": "AWS::IAM::Role",
      "Condition": "EnableRunnerDeprovision",
      "Properties": {
        "Tags": [{"Key": "runner.nuon.co/assumable", "Value": "true"}]
      }
    },
    "CustomNuonInstallIdDynamodbOperations": {
      "Type": "AWS::IAM::Role",
      "Condition": "EnableCustomNuonInstallIdDynamodbOperations",
      "Properties": {
        "Tags": [{"Key": "runner.nuon.co/assumable", "Value": "true"}]
      }
    },
    "CustomNuonInstallIdIngressOperations": {
      "Type": "AWS::IAM::Role",
      "Condition": "EnableCustomNuonInstallIdIngressOperations",
      "Properties": {
        "Tags": [{"Key": "runner.nuon.co/assumable", "Value": "true"}]
      }
    },
    "RunnerPhoneHomeRole": {
      "Type": "AWS::IAM::Role",
      "Properties": {
        "Tags": [{"Key": "Name", "Value": "phone-home"}]
      }
    }
  }
}`

func TestDiscoverAssumableRoles(t *testing.T) {
	roles := discoverAssumableRoles(mustDecodeTemplate(t, sampleAssumableTemplate))

	got := map[string]string{}
	for _, role := range roles {
		got[role.ParameterKey] = role.Label
	}

	want := map[string]string{
		enableRunnerMaintenanceParam:                  "Maintenance",
		enableRunnerProvisionParam:                    "Provision",
		enableRunnerDeprovisionParam:                  "Deprovision",
		"EnableCustomNuonInstallIdDynamodbOperations": "dynamodb-operations",
		"EnableCustomNuonInstallIdIngressOperations":  "ingress-operations",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d roles, got %v", len(want), got)
	}
	for key, label := range want {
		if got[key] != label {
			t.Fatalf("expected %s label %q, got %q", key, label, got[key])
		}
	}
}

func TestDiscoverAssumableRolesIncludesFirstClassWhenMissing(t *testing.T) {
	template := decodedTemplate{
		Parameters: map[string]struct {
			NoEcho bool `json:"NoEcho"`
		}{
			"EnableCustomOnly": {},
		},
		Resources: map[string]struct {
			Type       string `json:"Type"`
			Condition  string `json:"Condition"`
			Properties struct {
				Tags []cloudFormationTag `json:"Tags"`
			} `json:"Properties"`
		}{
			"CustomOnly": {
				Type:      "AWS::IAM::Role",
				Condition: "EnableCustomOnly",
				Properties: struct {
					Tags []cloudFormationTag `json:"Tags"`
				}{
					Tags: []cloudFormationTag{{Key: assumableRoleTag, Value: "true"}},
				},
			},
		},
	}

	roles := discoverAssumableRoles(template)
	got := map[string]struct{}{}
	for _, role := range roles {
		got[role.ParameterKey] = struct{}{}
	}

	for _, key := range []string{
		"EnableCustomOnly",
		enableRunnerMaintenanceParam,
		enableRunnerProvisionParam,
		enableRunnerDeprovisionParam,
	} {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected role %s, got %v", key, got)
		}
	}
}

func TestResolveRoleParameterValuesDisablesCustomRole(t *testing.T) {
	roles := discoverAssumableRoles(mustDecodeTemplate(t, sampleAssumableTemplate))
	values, err := resolveRoleParameterValues(roles, []string{"dynamodb-operations", "Deprovision"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if values["EnableCustomNuonInstallIdDynamodbOperations"] {
		t.Fatalf("expected dynamodb-operations to be disabled")
	}
	if values[enableRunnerDeprovisionParam] {
		t.Fatalf("expected deprovision to be disabled")
	}
	if !values[enableRunnerMaintenanceParam] || !values[enableRunnerProvisionParam] || !values["EnableCustomNuonInstallIdIngressOperations"] {
		t.Fatalf("expected remaining roles enabled, got %v", values)
	}
}

func TestResolveRoleParameterValuesMatchesParameterKey(t *testing.T) {
	roles := discoverAssumableRoles(mustDecodeTemplate(t, sampleAssumableTemplate))
	values, err := resolveRoleParameterValues(roles, []string{"EnableCustomNuonInstallIdIngressOperations"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if values["EnableCustomNuonInstallIdIngressOperations"] {
		t.Fatalf("expected ingress operations to be disabled")
	}
}

func TestResolveRoleParameterValuesUnknownRole(t *testing.T) {
	roles := discoverAssumableRoles(mustDecodeTemplate(t, sampleAssumableTemplate))
	_, err := resolveRoleParameterValues(roles, []string{"not-a-role"})
	if err == nil {
		t.Fatalf("expected unknown role error")
	}
	if !strings.Contains(err.Error(), "unknown role") || !strings.Contains(err.Error(), "dynamodb-operations") {
		t.Fatalf("expected unknown role error listing known labels, got %v", err)
	}
}

func TestResolveRoleParameterValuesRejectsAllDisabled(t *testing.T) {
	roles := firstClassAssumableRoles()
	_, err := resolveRoleParameterValues(roles, []string{"maintenance", "provision", "deprovision"})
	if err == nil {
		t.Fatalf("expected error when all roles are disabled")
	}
	if !strings.Contains(err.Error(), "at least one role") {
		t.Fatalf("expected at least one role error, got %v", err)
	}
}

func TestBuildStackParametersIncludesCustomRoleFlags(t *testing.T) {
	roles := discoverAssumableRoles(mustDecodeTemplate(t, sampleAssumableTemplate))
	values, err := resolveRoleParameterValues(roles, []string{"dynamodb-operations"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	params, err := buildStackParameters(
		"install",
		map[string]any{"foo": "value"},
		map[string]any{},
		values,
		map[string]struct{}{
			"ParameterFoo": {},
			"EnableCustomNuonInstallIdDynamodbOperations": {},
		},
		nil,
		nil,
		false,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	indexed := parametersByKey(params)
	if got := *indexed["EnableCustomNuonInstallIdDynamodbOperations"].ParameterValue; got != "false" {
		t.Fatalf("expected dynamodb role false, got %s", got)
	}
	if got := *indexed["EnableCustomNuonInstallIdIngressOperations"].ParameterValue; got != "true" {
		t.Fatalf("expected ingress role true, got %s", got)
	}
	if got := *indexed[enableRunnerMaintenanceParam].ParameterValue; got != "true" {
		t.Fatalf("expected maintenance true, got %s", got)
	}
}

func mustDecodeTemplate(t *testing.T, raw string) decodedTemplate {
	t.Helper()
	var template decodedTemplate
	if err := json.Unmarshal([]byte(raw), &template); err != nil {
		t.Fatalf("decode template fixture: %v", err)
	}
	return template
}
