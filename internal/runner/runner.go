package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	nuon "github.com/nuonco/nuon/sdks/nuon-go"
	"github.com/nuonco/nuon/sdks/nuon-go/models"

	"github.com/nuonco/nuon-ext-cf-stack/internal/config"
	"github.com/nuonco/nuon-ext-cf-stack/internal/debug"
	"github.com/nuonco/nuon-ext-cf-stack/internal/options"
	"github.com/nuonco/nuon-ext-cf-stack/internal/watch"
)

const (
	enableRunnerMaintenanceParam = "EnableRunnerMaintenance"
	enableRunnerProvisionParam   = "EnableRunnerProvision"
	enableRunnerDeprovisionParam = "EnableRunnerDeprovision"
	applyWaitTimeout             = 45 * time.Minute
)

type installStackMeta struct {
	InstallID    string
	InstallStack string
	TemplateURL  string
	StackName    string
	Region       string
	AccountID    string
}

type templateParameterMetadata struct {
	All    map[string]struct{}
	Secret map[string]struct{}
}

func Run(ctx context.Context, w io.Writer, operation string, opts options.CommonOptions) error {
	nuonEnv, err := config.LoadNuonEnv(nil)
	if err != nil {
		return fmt.Errorf("load nuon environment: %w", err)
	}

	apiClient, err := nuon.New(
		nuon.WithURL(nuonEnv.APIURL),
		nuon.WithAuthToken(nuonEnv.APIToken),
		nuon.WithOrgID(nuonEnv.OrgID),
	)
	if err != nil {
		return fmt.Errorf("create nuon client: %w", err)
	}

	install, err := apiClient.GetInstall(ctx, opts.InstallID)
	if err != nil {
		return fmt.Errorf("get install %q: %w", opts.InstallID, err)
	}

	if install.CloudPlatform != "" && !strings.EqualFold(install.CloudPlatform, "aws") {
		return fmt.Errorf("install %q uses cloud platform %q; only aws is supported", opts.InstallID, install.CloudPlatform)
	}

	installStack, err := apiClient.GetInstallStack(ctx, opts.InstallID)
	if err != nil {
		return fmt.Errorf("get install stack for %q: %w", opts.InstallID, err)
	}

	debug.Log("operation=%s install id=%s name=%q", operation, install.ID, install.Name)
	debug.Log("operation=%s stack id=%s status=%s", operation, installStack.ID, installStackStatus(installStack))

	meta, err := resolveInstallStackMeta(install, installStack)
	if err != nil {
		return err
	}

	templateMetadata, err := fetchTemplateParameterMetadata(ctx, meta.TemplateURL)
	if err != nil {
		return err
	}
	debug.Log(
		"operation=%s template parameter count=%d secret parameter count=%d",
		operation,
		len(templateMetadata.All),
		len(templateMetadata.Secret),
	)

	awsConfig, err := loadAWSConfig(ctx, meta.Region, opts.Profile)
	if err != nil {
		return err
	}

	if err := verifyAWSAccount(ctx, awsConfig, meta.AccountID); err != nil {
		return err
	}

	existingStackParameters, stackExists, err := fetchExistingStackParameterSet(ctx, awsConfig, meta.StackName)
	if err != nil {
		return err
	}
	debug.Log(
		"operation=%s stack exists=%t existing stack parameter count=%d",
		operation,
		stackExists,
		len(existingStackParameters),
	)

	parameters, err := buildStackParameters(
		operation,
		opts.Inputs,
		opts.Secrets,
		opts.Roles,
		templateMetadata.All,
		templateMetadata.Secret,
		existingStackParameters,
		stackExists,
	)
	if err != nil {
		return err
	}

	var action string
	applyMessage := fmt.Sprintf("CloudFormation update sent for stack %q. Applying...", meta.StackName)
	_, _ = fmt.Fprintf(w, "%s\n", applyMessage)

	apply := func(applyCtx context.Context) error {
		var applyErr error
		action, applyErr = applyStack(applyCtx, awsConfig, applyStackInput{
			StackName:   meta.StackName,
			TemplateURL: meta.TemplateURL,
			Parameters:  parameters,
			Tags: []cftypes.Tag{
				{Key: aws.String("nuon-install-id"), Value: aws.String(meta.InstallID)},
				{Key: aws.String("nuon-install-stack-id"), Value: aws.String(meta.InstallStack)},
			},
		})
		return applyErr
	}

	if opts.Watch {
		if err := watch.Run(ctx, w, applyMessage, apply); err != nil {
			return err
		}
	} else {
		if err := apply(ctx); err != nil {
			return err
		}
	}

	stackURL := stackConsoleURL(meta.Region, meta.StackName)

	_, _ = fmt.Fprintf(
		w,
		"operation: %s\nresult: %s\nstack: %q\nregion: %s\ntemplate: %s\nstack_url: %s\n",
		operation,
		action,
		meta.StackName,
		meta.Region,
		meta.TemplateURL,
		stackURL,
	)

	return nil
}

func resolveInstallStackMeta(install *models.AppInstall, installStack *models.AppInstallStack) (*installStackMeta, error) {
	if install == nil {
		return nil, fmt.Errorf("install is required")
	}
	if installStack == nil {
		return nil, fmt.Errorf("install %q does not have a stack record", install.ID)
	}
	if installStack.InstallID != "" && installStack.InstallID != install.ID {
		return nil, fmt.Errorf("install stack %q belongs to %q, not %q", installStack.ID, installStack.InstallID, install.ID)
	}

	version := latestInstallStackVersion(installStack.Versions)
	if version == nil {
		return nil, fmt.Errorf("install %q has no stack versions", install.ID)
	}

	templateURL := strings.TrimSpace(version.TemplateURL)
	if templateURL == "" {
		templateURL = quickCreateValue(version.QuickLinkURL, "templateUrl")
	}
	if templateURL == "" {
		return nil, fmt.Errorf("install %q stack version %q has no template URL", install.ID, version.ID)
	}

	stackName := quickCreateValue(version.QuickLinkURL, "stackName")
	if stackName == "" {
		return nil, fmt.Errorf("install %q stack version %q has no stackName in quick link", install.ID, version.ID)
	}

	region := ""
	accountID := ""
	if installStack.InstallStackOutputs != nil && installStack.InstallStackOutputs.Aws != nil {
		region = strings.TrimSpace(installStack.InstallStackOutputs.Aws.Region)
		accountID = strings.TrimSpace(installStack.InstallStackOutputs.Aws.AccountID)
	}
	if region == "" && install.AwsAccount != nil {
		region = strings.TrimSpace(install.AwsAccount.Region)
	}
	if region == "" {
		return nil, fmt.Errorf("unable to resolve aws region from install %q stack outputs", install.ID)
	}

	return &installStackMeta{
		InstallID:    install.ID,
		InstallStack: installStack.ID,
		TemplateURL:  templateURL,
		StackName:    stackName,
		Region:       region,
		AccountID:    accountID,
	}, nil
}

func latestInstallStackVersion(versions []*models.AppInstallStackVersion) *models.AppInstallStackVersion {
	if len(versions) == 0 {
		return nil
	}

	copyVersions := make([]*models.AppInstallStackVersion, 0, len(versions))
	for _, version := range versions {
		if version != nil {
			copyVersions = append(copyVersions, version)
		}
	}
	if len(copyVersions) == 0 {
		return nil
	}

	sort.Slice(copyVersions, func(i, j int) bool {
		return parseVersionTime(copyVersions[i]).After(parseVersionTime(copyVersions[j]))
	})

	return copyVersions[0]
}

func installStackStatus(stack *models.AppInstallStack) string {
	if stack == nil {
		return "unknown"
	}

	latest := latestInstallStackVersion(stack.Versions)
	if latest == nil {
		return "unknown"
	}

	if latest.CompositeStatus != nil && latest.CompositeStatus.Status != "" {
		return string(latest.CompositeStatus.Status)
	}

	if latest.CompositeStatus != nil && latest.CompositeStatus.StatusHumanDescription != "" {
		return latest.CompositeStatus.StatusHumanDescription
	}

	return "unknown"
}

func parseVersionTime(version *models.AppInstallStackVersion) time.Time {
	if version == nil {
		return time.Time{}
	}

	for _, candidate := range []string{version.UpdatedAt, version.CreatedAt} {
		if ts, err := time.Parse(time.RFC3339, candidate); err == nil {
			return ts
		}
	}

	return time.Time{}
}

func quickCreateValue(quickLinkURL, key string) string {
	if strings.TrimSpace(quickLinkURL) == "" {
		return ""
	}

	parsedURL, err := url.Parse(quickLinkURL)
	if err != nil {
		return ""
	}

	fragment := parsedURL.Fragment
	qMarkIdx := strings.Index(fragment, "?")
	if qMarkIdx < 0 || qMarkIdx == len(fragment)-1 {
		return ""
	}

	values, err := url.ParseQuery(fragment[qMarkIdx+1:])
	if err != nil {
		return ""
	}

	return strings.TrimSpace(values.Get(key))
}

func stackConsoleURL(region, stackName string) string {
	cleanRegion := strings.TrimSpace(region)
	cleanStackName := strings.TrimSpace(stackName)
	if cleanRegion == "" || cleanStackName == "" {
		return ""
	}

	fragmentParams := url.Values{}
	fragmentParams.Set("filteringStatus", "active")
	fragmentParams.Set("filteringText", cleanStackName)
	fragmentParams.Set("hideStacks", "false")
	fragmentParams.Set("viewNested", "true")

	return fmt.Sprintf(
		"https://%s.console.aws.amazon.com/cloudformation/home?region=%s#/stacks?%s",
		cleanRegion,
		url.QueryEscape(cleanRegion),
		fragmentParams.Encode(),
	)
}

func fetchTemplateParameterMetadata(ctx context.Context, templateURL string) (templateParameterMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, templateURL, nil)
	if err != nil {
		return templateParameterMetadata{}, fmt.Errorf("create template request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return templateParameterMetadata{}, fmt.Errorf("fetch template %s: %w", templateURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return templateParameterMetadata{}, fmt.Errorf("fetch template %s: unexpected status %d", templateURL, resp.StatusCode)
	}

	var template struct {
		Parameters map[string]struct {
			NoEcho bool `json:"NoEcho"`
		} `json:"Parameters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&template); err != nil {
		return templateParameterMetadata{}, fmt.Errorf("decode template %s: %w", templateURL, err)
	}

	all := make(map[string]struct{}, len(template.Parameters))
	secret := map[string]struct{}{}
	for key, parameter := range template.Parameters {
		all[key] = struct{}{}
		if parameter.NoEcho {
			secret[key] = struct{}{}
		}
	}

	return templateParameterMetadata{
		All:    all,
		Secret: secret,
	}, nil
}

func loadAWSConfig(ctx context.Context, region, profile string) (aws.Config, error) {
	options := []func(*awscfg.LoadOptions) error{awscfg.WithRegion(region)}
	if profile != "" {
		options = append(options, awscfg.WithSharedConfigProfile(profile))
	}

	awsConfig, err := awscfg.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config (region=%s profile=%q): %w", region, profile, err)
	}

	return awsConfig, nil
}

func verifyAWSAccount(ctx context.Context, awsConfig aws.Config, expectedAccountID string) error {
	if expectedAccountID == "" {
		return nil
	}

	stsClient := sts.NewFromConfig(awsConfig)
	id, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("verify aws account: %w", err)
	}

	actualAccountID := strings.TrimSpace(aws.ToString(id.Account))
	if actualAccountID == "" {
		return fmt.Errorf("verify aws account: caller identity did not return an account id")
	}

	if actualAccountID != expectedAccountID {
		return fmt.Errorf("aws account mismatch: expected %s from install stack outputs, got %s from current credentials", expectedAccountID, actualAccountID)
	}

	return nil
}

func fetchExistingStackParameterSet(ctx context.Context, awsConfig aws.Config, stackName string) (map[string]struct{}, bool, error) {
	client := cloudformation.NewFromConfig(awsConfig)
	output, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)})
	if err != nil {
		if isStackNotFoundError(err) {
			return map[string]struct{}{}, false, nil
		}
		return nil, false, fmt.Errorf("describe stack %q for parameter reuse: %w", stackName, err)
	}

	if len(output.Stacks) == 0 {
		return map[string]struct{}{}, false, nil
	}

	stack := output.Stacks[0]
	keys := make(map[string]struct{}, len(stack.Parameters))
	for _, parameter := range stack.Parameters {
		key := strings.TrimSpace(aws.ToString(parameter.ParameterKey))
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}

	return keys, true, nil
}

func buildStackParameters(
	operation string,
	inputs, secrets map[string]any,
	roles options.RoleOptions,
	templateParameters map[string]struct{},
	templateSecretParameters map[string]struct{},
	existingStackParameters map[string]struct{},
	stackExists bool,
) ([]cftypes.Parameter, error) {
	merged := map[string]string{}
	origin := map[string]string{}

	if err := mergeInputParameters(merged, origin, inputs, templateParameters); err != nil {
		return nil, err
	}
	if err := mergeParameters(merged, origin, secrets, "secrets"); err != nil {
		return nil, err
	}

	merged[enableRunnerMaintenanceParam] = strconv.FormatBool(roles.Maintenance)
	origin[enableRunnerMaintenanceParam] = "roles"
	merged[enableRunnerProvisionParam] = strconv.FormatBool(roles.Provision)
	origin[enableRunnerProvisionParam] = "roles"
	merged[enableRunnerDeprovisionParam] = strconv.FormatBool(roles.Deprovision)
	origin[enableRunnerDeprovisionParam] = "roles"

	parametersByKey := make(map[string]cftypes.Parameter, len(merged)+len(templateSecretParameters))
	for key, value := range merged {
		parametersByKey[key] = cftypes.Parameter{
			ParameterKey:   aws.String(key),
			ParameterValue: aws.String(value),
		}
	}

	if shouldReuseExistingSecretValues(operation, stackExists) {
		applySecretParameterBehavior(parametersByKey, origin, templateSecretParameters, existingStackParameters, operation)
	}

	keys := make([]string, 0, len(parametersByKey))
	for key := range parametersByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	params := make([]cftypes.Parameter, 0, len(keys))
	for _, key := range keys {
		params = append(params, parametersByKey[key])
	}

	return params, nil
}

func applySecretParameterBehavior(
	parametersByKey map[string]cftypes.Parameter,
	origin map[string]string,
	templateSecretParameters map[string]struct{},
	existingStackParameters map[string]struct{},
	operation string,
) {
	secretKeys := make([]string, 0, len(templateSecretParameters))
	for key := range templateSecretParameters {
		secretKeys = append(secretKeys, key)
	}
	sort.Strings(secretKeys)

	for _, key := range secretKeys {
		if source, exists := origin[key]; exists {
			parameter := parametersByKey[key]
			if source == "secrets" && isEmptyStringParameterValue(parameter) {
				if _, exists := existingStackParameters[key]; exists {
					parametersByKey[key] = cftypes.Parameter{
						ParameterKey:     aws.String(key),
						UsePreviousValue: aws.Bool(true),
					}
					origin[key] = "existing-stack"
					debug.Info("operation=%s secret parameter %q: empty value provided; keeping existing stack value", operation, key)
					continue
				}

				debug.Log("operation=%s secret parameter %q: empty value provided but no existing stack value to keep", operation, key)
				continue
			}

			debug.Log("operation=%s secret parameter %q: updated from provided %s", operation, key, source)
			continue
		}

		if _, exists := existingStackParameters[key]; exists {
			parametersByKey[key] = cftypes.Parameter{
				ParameterKey:     aws.String(key),
				UsePreviousValue: aws.Bool(true),
			}
			origin[key] = "existing-stack"
			debug.Info("operation=%s secret parameter %q: keeping existing stack value", operation, key)
			continue
		}

		debug.Log("operation=%s secret parameter %q: no provided value and no existing stack value to keep", operation, key)
	}
}

func shouldReuseExistingSecretValues(operation string, stackExists bool) bool {
	if isUpgradeOperation(operation) {
		return true
	}

	// Install can be re-run against an existing stack, which uses UpdateStack semantics.
	return stackExists
}

func isEmptyStringParameterValue(parameter cftypes.Parameter) bool {
	if parameter.ParameterValue == nil {
		return false
	}

	return *parameter.ParameterValue == ""
}

func isUpgradeOperation(operation string) bool {
	return strings.EqualFold(strings.TrimSpace(operation), "upgrade")
}

func mergeInputParameters(target, origin map[string]string, inputs map[string]any, templateParameters map[string]struct{}) error {
	inputKeys := make([]string, 0, len(inputs))
	for key := range inputs {
		inputKeys = append(inputKeys, key)
	}
	sort.Strings(inputKeys)

	for _, rawKey := range inputKeys {
		rawValue := inputs[rawKey]
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return fmt.Errorf("inputs contains an empty parameter key")
		}

		parameterKey, found, candidates := resolveInputParameterKey(key, templateParameters)
		if !found {
			debug.Info("omitting input %q: no matching stack parameter (candidates=%s)", key, strings.Join(candidates, ","))
			continue
		}

		if parameterKey != key {
			debug.Log("mapped input %q -> %q", key, parameterKey)
		}

		if previousSource, exists := origin[parameterKey]; exists {
			return fmt.Errorf("duplicate parameter key %q in %s and inputs", parameterKey, previousSource)
		}

		value, err := toParameterValue(rawValue)
		if err != nil {
			return fmt.Errorf("convert input parameter %q: %w", key, err)
		}

		target[parameterKey] = value
		origin[parameterKey] = "inputs"
	}

	return nil
}

func resolveInputParameterKey(inputKey string, templateParameters map[string]struct{}) (string, bool, []string) {
	candidates := inputParameterCandidates(inputKey)
	for _, candidate := range candidates {
		if _, exists := templateParameters[candidate]; exists {
			return candidate, true, candidates
		}
	}

	return "", false, candidates
}

func inputParameterCandidates(inputKey string) []string {
	candidates := []string{inputKey}
	pascal := toPascalCase(inputKey)
	if pascal != "" {
		candidates = appendUnique(candidates, "Parameter"+pascal)
		candidates = appendUnique(candidates, pascal)
	}

	return candidates
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}

	return append(values, candidate)
}

func toPascalCase(value string) string {
	var b strings.Builder
	upperNext := true

	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if upperNext {
				b.WriteRune(unicode.ToUpper(r))
				upperNext = false
				continue
			}
			b.WriteRune(r)
			continue
		}

		upperNext = true
	}

	return b.String()
}

func mergeParameters(target, origin map[string]string, source map[string]any, sourceName string) error {
	for rawKey, rawValue := range source {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return fmt.Errorf("%s contains an empty parameter key", sourceName)
		}

		if previousSource, exists := origin[key]; exists {
			return fmt.Errorf("duplicate parameter key %q in %s and %s", key, previousSource, sourceName)
		}

		value, err := toParameterValue(rawValue)
		if err != nil {
			return fmt.Errorf("convert %s parameter %q: %w", sourceName, key, err)
		}

		target[key] = value
		origin[key] = sourceName
	}

	return nil
}

func toParameterValue(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", typed), nil
	default:
		jsonValue, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(jsonValue), nil
	}
}

type applyStackInput struct {
	StackName   string
	TemplateURL string
	Parameters  []cftypes.Parameter
	Tags        []cftypes.Tag
}

func applyStack(ctx context.Context, awsConfig aws.Config, input applyStackInput) (string, error) {
	client := cloudformation.NewFromConfig(awsConfig)

	exists, err := stackExists(ctx, client, input.StackName)
	if err != nil {
		return "", err
	}

	capabilities := []cftypes.Capability{
		cftypes.CapabilityCapabilityIam,
		cftypes.CapabilityCapabilityNamedIam,
		cftypes.CapabilityCapabilityAutoExpand,
	}

	if !exists {
		createOutput, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
			StackName:    aws.String(input.StackName),
			TemplateURL:  aws.String(input.TemplateURL),
			Parameters:   input.Parameters,
			Capabilities: capabilities,
			Tags:         input.Tags,
		})
		if err != nil {
			return "", fmt.Errorf("create cloudformation stack %q: %w", input.StackName, err)
		}

		waiter := cloudformation.NewStackCreateCompleteWaiter(client)
		if err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{StackName: createOutput.StackId}, applyWaitTimeout); err != nil {
			return "", fmt.Errorf("wait for stack %q create completion: %w", input.StackName, err)
		}

		return "created", nil
	}

	_, err = client.UpdateStack(ctx, &cloudformation.UpdateStackInput{
		StackName:    aws.String(input.StackName),
		TemplateURL:  aws.String(input.TemplateURL),
		Parameters:   input.Parameters,
		Capabilities: capabilities,
		Tags:         input.Tags,
	})
	if err != nil {
		if isNoUpdatesError(err) {
			return "unchanged", nil
		}
		return "", fmt.Errorf("update cloudformation stack %q: %w", input.StackName, err)
	}

	waiter := cloudformation.NewStackUpdateCompleteWaiter(client)
	if err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(input.StackName)}, applyWaitTimeout); err != nil {
		return "", fmt.Errorf("wait for stack %q update completion: %w", input.StackName, err)
	}

	return "updated", nil
}

func stackExists(ctx context.Context, client *cloudformation.Client, stackName string) (bool, error) {
	_, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)})
	if err != nil {
		if isStackNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("describe stack %q: %w", stackName, err)
	}

	return true, nil
}

func isStackNotFoundError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "ValidationError" && strings.Contains(strings.ToLower(apiErr.ErrorMessage()), "does not exist") {
			return true
		}
	}

	return strings.Contains(strings.ToLower(err.Error()), "does not exist")
}

func isNoUpdatesError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "ValidationError" && strings.Contains(strings.ToLower(apiErr.ErrorMessage()), "no updates are to be performed") {
			return true
		}
	}

	return strings.Contains(strings.ToLower(err.Error()), "no updates are to be performed")
}
