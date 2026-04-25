package lsp

import (
	"encoding/json"
	"fmt"
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
}

type initializeOptionsFile struct {
	SearchPaths            []string                                  `json:"searchPaths,omitempty"`
	DelphiInstallationPath string                                    `json:"delphiInstallationPath,omitempty"`
	WorkspaceSettings      map[string]WorkspaceInitializationOptions `json:"workspaceSettings,omitempty"`
}

// ParseInitializeOptionsJSON parses and normalizes initialization options from a JSON payload.
func ParseInitializeOptionsJSON(content []byte) (InitializeOptions, error) {
	var parsed initializeOptionsFile
	if err := json.Unmarshal(content, &parsed); err != nil {
		return InitializeOptions{}, err
	}

	options := InitializeOptions{
		SearchPaths:            append([]string(nil), parsed.SearchPaths...),
		DelphiInstallationPath: parsed.DelphiInstallationPath,
	}

	if len(parsed.WorkspaceSettings) > 0 {
		options.WorkspaceSettings = make(map[string]WorkspaceInitializationOptions, len(parsed.WorkspaceSettings))
		for root, settings := range parsed.WorkspaceSettings {
			options.WorkspaceSettings[root] = WorkspaceInitializationOptions{
				SearchPaths:            append([]string(nil), settings.SearchPaths...),
				DelphiInstallationPath: settings.DelphiInstallationPath,
			}
		}
	}

	return NormalizeInitializeOptions(options)
}

// NormalizeInitializeOptions trims empty values, normalizes workspace root keys to file:// URIs,
// and omits empty arrays/maps when possible.
func NormalizeInitializeOptions(options InitializeOptions) (InitializeOptions, error) {
	normalized := InitializeOptions{
		SearchPaths: normalizeNonEmptyStrings(options.SearchPaths),
	}

	normalizedDelphiInstallationPath, err := normalizeDelphiInstallationPath(options.DelphiInstallationPath)
	if err != nil {
		return InitializeOptions{}, err
	}
	normalized.DelphiInstallationPath = normalizedDelphiInstallationPath

	if len(options.WorkspaceSettings) > 0 {
		normalized.WorkspaceSettings = make(map[string]WorkspaceInitializationOptions)
		for root, settings := range options.WorkspaceSettings {
			normalizedRoot, ok, err := normalizeWorkspaceSettingsRoot(root)
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
			}

			if len(normalizedSettings.SearchPaths) == 0 && normalizedSettings.DelphiInstallationPath == "" {
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

func normalizeWorkspaceSettingsRoot(root string) (string, bool, error) {
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
