package runner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const assumableRoleTag = "runner.nuon.co/assumable"

type assumableRole struct {
	ParameterKey string
	Label        string
}

type parameterLabel struct {
	Value string
}

func (p *parameterLabel) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		p.Value = asString
		return nil
	}

	var asObject struct {
		Default string `json:"default"`
	}
	if err := json.Unmarshal(data, &asObject); err != nil {
		return err
	}
	p.Value = asObject.Default
	return nil
}

type decodedTemplate struct {
	Parameters map[string]struct {
		NoEcho bool `json:"NoEcho"`
	} `json:"Parameters"`
	Metadata struct {
		Interface struct {
			ParameterLabels map[string]parameterLabel `json:"ParameterLabels"`
		} `json:"AWS::CloudFormation::Interface"`
	} `json:"Metadata"`
	Resources map[string]struct {
		Type       string `json:"Type"`
		Condition  string `json:"Condition"`
		Properties struct {
			Tags []cloudFormationTag `json:"Tags"`
		} `json:"Properties"`
	} `json:"Resources"`
}

type cloudFormationTag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

func firstClassAssumableRoles() []assumableRole {
	return []assumableRole{
		{ParameterKey: enableRunnerMaintenanceParam, Label: "maintenance"},
		{ParameterKey: enableRunnerProvisionParam, Label: "provision"},
		{ParameterKey: enableRunnerDeprovisionParam, Label: "deprovision"},
	}
}

func discoverAssumableRoles(template decodedTemplate) []assumableRole {
	labels := template.Metadata.Interface.ParameterLabels
	discovered := make(map[string]assumableRole)

	for _, resource := range template.Resources {
		if resource.Type != "AWS::IAM::Role" {
			continue
		}
		if !hasAssumableTag(resource.Properties.Tags) {
			continue
		}

		parameterKey := strings.TrimSpace(resource.Condition)
		if parameterKey == "" {
			continue
		}
		if template.Parameters != nil {
			if _, exists := template.Parameters[parameterKey]; !exists {
				continue
			}
		}

		label := parameterKey
		if named, ok := labels[parameterKey]; ok && strings.TrimSpace(named.Value) != "" {
			label = strings.TrimSpace(named.Value)
		}
		discovered[parameterKey] = assumableRole{
			ParameterKey: parameterKey,
			Label:        label,
		}
	}

	roles := make([]assumableRole, 0, len(discovered)+3)
	seen := map[string]struct{}{}
	for _, role := range discovered {
		roles = append(roles, role)
		seen[role.ParameterKey] = struct{}{}
	}
	for _, role := range firstClassAssumableRoles() {
		if _, exists := seen[role.ParameterKey]; exists {
			continue
		}
		if named, ok := labels[role.ParameterKey]; ok && strings.TrimSpace(named.Value) != "" {
			role.Label = strings.TrimSpace(named.Value)
		}
		roles = append(roles, role)
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].ParameterKey < roles[j].ParameterKey
	})

	return roles
}

func hasAssumableTag(tags []cloudFormationTag) bool {
	for _, tag := range tags {
		if tag.Key == assumableRoleTag && strings.EqualFold(strings.TrimSpace(tag.Value), "true") {
			return true
		}
	}
	return false
}

func resolveRoleParameterValues(roles []assumableRole, disabled []string) (map[string]bool, error) {
	if len(roles) == 0 {
		roles = firstClassAssumableRoles()
	}

	values := make(map[string]bool, len(roles))
	for _, role := range roles {
		values[role.ParameterKey] = true
	}

	var unknown []string
	for _, name := range disabled {
		matches := matchingRoles(roles, name)
		if len(matches) == 0 {
			unknown = append(unknown, name)
			continue
		}
		if len(matches) > 1 {
			names := make([]string, 0, len(matches))
			for _, role := range matches {
				names = append(names, roleDisplayName(role))
			}
			sort.Strings(names)
			return nil, fmt.Errorf("role %q is ambiguous; matches %s", name, strings.Join(names, ", "))
		}
		values[matches[0].ParameterKey] = false
	}

	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown role %s; known roles: %s", strings.Join(quoteNames(unknown), ", "), strings.Join(knownRoleNames(roles), ", "))
	}

	enabled := 0
	for _, enabledValue := range values {
		if enabledValue {
			enabled++
		}
	}
	if enabled == 0 {
		return nil, fmt.Errorf("at least one role must remain enabled")
	}

	return values, nil
}

func matchingRoles(roles []assumableRole, name string) []assumableRole {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return nil
	}

	var matches []assumableRole
	for _, role := range roles {
		if roleMatchesName(role, needle) {
			matches = append(matches, role)
		}
	}
	return matches
}

func roleMatchesName(role assumableRole, name string) bool {
	if strings.EqualFold(role.ParameterKey, name) {
		return true
	}
	if strings.EqualFold(role.Label, name) {
		return true
	}
	return false
}

func knownRoleNames(roles []assumableRole) []string {
	names := make([]string, 0, len(roles))
	seen := map[string]struct{}{}
	for _, role := range roles {
		name := roleDisplayName(role)
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func roleDisplayName(role assumableRole) string {
	if strings.TrimSpace(role.Label) != "" {
		return role.Label
	}
	return role.ParameterKey
}

func quoteNames(names []string) []string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	return quoted
}
