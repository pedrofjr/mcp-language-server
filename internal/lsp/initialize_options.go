package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// WorkspaceInitializationOptions defines initialization options scoped to a workspace root.
//
// Example:
//
//	{"searchPaths":["Source"],"delphiInstallationPath":"C:/Delphi6"}
type WorkspaceInitializationOptions struct {
	SearchPaths            []string `json:"searchPaths,omitempty"`
	DelphiInstallationPath string   `json:"delphiInstallationPath,omitempty"`
	extraFields            map[string]json.RawMessage
}

// InitializeOptions defines initialization options sent through initialize.initializationOptions.
//
// Example:
//
//	{"searchPaths":["Lib"],"workspaceSettings":{"file:///C:/Repo":{"searchPaths":["Source"]}}}
type InitializeOptions struct {
	SearchPaths            []string                                  `json:"searchPaths,omitempty"`
	DelphiInstallationPath string                                    `json:"delphiInstallationPath,omitempty"`
	WorkspaceSettings      map[string]WorkspaceInitializationOptions `json:"workspaceSettings,omitempty"`
	extraFields            map[string]json.RawMessage
}

// ParseInitializeOptionsJSON parses and normalizes initialization options from a JSON payload.
func ParseInitializeOptionsJSON(content []byte) (InitializeOptions, error) {
	return ParseInitializeOptionsJSONWithBaseDir(content, "")
}

// ParseInitializeOptionsJSONWithBaseDir parses and normalizes initialization options from a JSON
// payload, resolving relative workspaceSettings roots against workspaceSettingsBaseDir when set.
func ParseInitializeOptionsJSONWithBaseDir(content []byte, workspaceSettingsBaseDir string) (InitializeOptions, error) {
	var rawOptions map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawOptions); err != nil {
		return InitializeOptions{}, err
	}

	options := InitializeOptions{
		extraFields: extractUnknownRawFields(rawOptions, "searchPaths", "delphiInstallationPath", "workspaceSettings"),
	}

	searchPaths, err := unmarshalStringSliceField(rawOptions, "searchPaths")
	if err != nil {
		return InitializeOptions{}, err
	}
	options.SearchPaths = searchPaths

	delphiInstallationPath, err := unmarshalStringField(rawOptions, "delphiInstallationPath")
	if err != nil {
		return InitializeOptions{}, err
	}
	options.DelphiInstallationPath = delphiInstallationPath

	workspaceSettings, err := parseWorkspaceSettingsJSON(rawOptions)
	if err != nil {
		return InitializeOptions{}, err
	}
	options.WorkspaceSettings = workspaceSettings

	return normalizeInitializeOptions(options, workspaceSettingsBaseDir)
}

// NormalizeInitializeOptions trims empty values, normalizes workspace root keys to file:// URIs,
// and omits empty arrays/maps when possible.
func NormalizeInitializeOptions(options InitializeOptions) (InitializeOptions, error) {
	return normalizeInitializeOptions(options, "")
}

func normalizeInitializeOptions(options InitializeOptions, workspaceSettingsBaseDir string) (InitializeOptions, error) {
	normalized := InitializeOptions{
		SearchPaths: normalizeNonEmptyStrings(options.SearchPaths),
		extraFields: cloneRawMessages(options.extraFields),
	}

	normalizedDelphiInstallationPath, err := normalizeDelphiInstallationPath(options.DelphiInstallationPath)
	if err != nil {
		return InitializeOptions{}, err
	}
	normalized.DelphiInstallationPath = normalizedDelphiInstallationPath

	if len(options.WorkspaceSettings) > 0 {
		normalized.WorkspaceSettings = make(map[string]WorkspaceInitializationOptions)
		for root, settings := range options.WorkspaceSettings {
			normalizedRoot, ok, err := normalizeWorkspaceSettingsRoot(root, workspaceSettingsBaseDir)
			if err != nil {
				return InitializeOptions{}, err
			}
			if !ok {
				continue
			}

			normalizedWorkspaceDelphiInstallationPath, err := normalizeDelphiInstallationPath(settings.DelphiInstallationPath)
			if err != nil {
				return InitializeOptions{}, fmt.Errorf("workspaceSettings key %q has invalid delphiInstallationPath: %w", root, err)
			}

			normalizedSettings := WorkspaceInitializationOptions{
				SearchPaths:            normalizeNonEmptyStrings(settings.SearchPaths),
				DelphiInstallationPath: normalizedWorkspaceDelphiInstallationPath,
				extraFields:            cloneRawMessages(settings.extraFields),
			}

			if len(normalizedSettings.SearchPaths) == 0 && normalizedSettings.DelphiInstallationPath == "" && len(normalizedSettings.extraFields) == 0 {
				continue
			}

			normalized.WorkspaceSettings[normalizedRoot] = normalizedSettings
		}

		if len(normalized.WorkspaceSettings) == 0 {
			normalized.WorkspaceSettings = nil
		}
	}

	return normalized, nil
}

func parseWorkspaceSettingsJSON(rawOptions map[string]json.RawMessage) (map[string]WorkspaceInitializationOptions, error) {
	rawWorkspaceSettings, ok := rawOptions["workspaceSettings"]
	if !ok {
		return nil, nil
	}

	var workspaceSettingsJSON map[string]json.RawMessage
	if err := json.Unmarshal(rawWorkspaceSettings, &workspaceSettingsJSON); err != nil {
		return nil, err
	}
	if len(workspaceSettingsJSON) == 0 {
		return nil, nil
	}

	workspaceSettings := make(map[string]WorkspaceInitializationOptions, len(workspaceSettingsJSON))
	for root, rawSettings := range workspaceSettingsJSON {
		var settingFields map[string]json.RawMessage
		if err := json.Unmarshal(rawSettings, &settingFields); err != nil {
			return nil, fmt.Errorf("workspaceSettings key %q: %w", root, err)
		}

		searchPaths, err := unmarshalStringSliceField(settingFields, "searchPaths")
		if err != nil {
			return nil, fmt.Errorf("workspaceSettings key %q: %w", root, err)
		}

		delphiInstallationPath, err := unmarshalStringField(settingFields, "delphiInstallationPath")
		if err != nil {
			return nil, fmt.Errorf("workspaceSettings key %q: %w", root, err)
		}

		workspaceSettings[root] = WorkspaceInitializationOptions{
			SearchPaths:            searchPaths,
			DelphiInstallationPath: delphiInstallationPath,
			extraFields:            extractUnknownRawFields(settingFields, "searchPaths", "delphiInstallationPath"),
		}
	}

	return workspaceSettings, nil
}

func unmarshalStringSliceField(fields map[string]json.RawMessage, fieldName string) ([]string, error) {
	rawValue, ok := fields[fieldName]
	if !ok {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal(rawValue, &values); err != nil {
		return nil, err
	}

	return values, nil
}

func unmarshalStringField(fields map[string]json.RawMessage, fieldName string) (string, error) {
	rawValue, ok := fields[fieldName]
	if !ok {
		return "", nil
	}

	var value string
	if err := json.Unmarshal(rawValue, &value); err != nil {
		return "", err
	}

	return value, nil
}

func extractUnknownRawFields(fields map[string]json.RawMessage, knownFieldNames ...string) map[string]json.RawMessage {
	if len(fields) == 0 {
		return nil
	}

	knownFields := make(map[string]struct{}, len(knownFieldNames))
	for _, fieldName := range knownFieldNames {
		knownFields[fieldName] = struct{}{}
	}

	unknownFields := make(map[string]json.RawMessage)
	for fieldName, rawValue := range fields {
		if _, isKnownField := knownFields[fieldName]; isKnownField {
			continue
		}

		unknownFields[fieldName] = append(json.RawMessage(nil), rawValue...)
	}

	if len(unknownFields) == 0 {
		return nil
	}

	return unknownFields
}

func cloneRawMessages(values map[string]json.RawMessage) map[string]json.RawMessage {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		cloned[key] = append(json.RawMessage(nil), value...)
	}

	return cloned
}

func normalizeNonEmptyStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeWorkspaceSettingsRoot(root string, workspaceSettingsBaseDir string) (string, bool, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", false, nil
	}

	parsedURI, isFileURI, err := parseCaseInsensitiveFileURI(trimmed)
	if err != nil {
		return "", false, fmt.Errorf("invalid workspaceSettings file URI %q: %w", root, err)
	}
	if isFileURI {
		return string(parsedURI), true, nil
	}

	if strings.Contains(trimmed, "://") {
		return "", false, fmt.Errorf("workspaceSettings key %q must be a local path or file:// URI", root)
	}

	if workspaceSettingsBaseDir != "" && !filepath.IsAbs(trimmed) {
		trimmed = filepath.Join(workspaceSettingsBaseDir, trimmed)
	}

	return string(protocol.URIFromPath(trimmed)), true, nil
}

func normalizeDelphiInstallationPath(pathValue string) (string, error) {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "", nil
	}

	parsedURI, isFileURI, err := parseCaseInsensitiveFileURI(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid file URI %q: %w", pathValue, err)
	}
	if !isFileURI {
		return trimmed, nil
	}

	localPath, err := safeDocumentURIPath(parsedURI)
	if err != nil {
		return "", fmt.Errorf("invalid file URI %q: %w", pathValue, err)
	}

	return localPath, nil
}

func parseCaseInsensitiveFileURI(value string) (protocol.DocumentUri, bool, error) {
	schemeSeparatorIndex := strings.Index(value, "://")
	if schemeSeparatorIndex < 0 {
		return "", false, nil
	}

	scheme := value[:schemeSeparatorIndex]
	if !strings.EqualFold(scheme, "file") {
		return "", false, nil
	}

	canonicalURI := "file" + value[schemeSeparatorIndex:]
	parsed, err := protocol.ParseDocumentUri(canonicalURI)
	if err != nil {
		return "", true, err
	}

	return parsed, true, nil
}
