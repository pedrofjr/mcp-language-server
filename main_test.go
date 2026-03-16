package main

import "testing"

func TestInitialOmniPascalConfigDefaultsToDelphi(t *testing.T) {
	cfg := config{
		workspaceDir:                     `C:\workspace`,
		omnipascalDelphiInstallationPath: `C:\Delphi6`,
	}

	result, err := cfg.initialOmniPascalConfig()
	if err != nil {
		t.Fatalf("initialOmniPascalConfig() error = %v", err)
	}

	if got := result["workspacePaths"]; got != `C:\workspace` {
		t.Fatalf("workspacePaths = %v, want %q", got, `C:\workspace`)
	}

	if got := result["delphiInstallationPath"]; got != `C:\Delphi6` {
		t.Fatalf("delphiInstallationPath = %v, want %q", got, `C:\Delphi6`)
	}

	if got := result["defaultDevelopmentEnvironment"]; got != "Delphi" {
		t.Fatalf("defaultDevelopmentEnvironment = %v, want %q", got, "Delphi")
	}
}

func TestInitialOmniPascalConfigMergesJSONAndFlags(t *testing.T) {
	cfg := config{
		workspaceDir:                     `C:\workspace`,
		omnipascalConfigJSON:             `{"workspacePaths":"C:\\wrong","searchPath":"from-json","symbolIndex":"workspace"}`,
		omnipascalDelphiInstallationPath: `C:\Delphi6`,
		omnipascalSearchPath:             `C:\lib;C:\vcl`,
	}

	result, err := cfg.initialOmniPascalConfig()
	if err != nil {
		t.Fatalf("initialOmniPascalConfig() error = %v", err)
	}

	if got := result["workspacePaths"]; got != `C:\workspace` {
		t.Fatalf("workspacePaths = %v, want %q", got, `C:\workspace`)
	}

	if got := result["searchPath"]; got != `C:\lib;C:\vcl` {
		t.Fatalf("searchPath = %v, want %q", got, `C:\lib;C:\vcl`)
	}

	if got := result["symbolIndex"]; got != "workspace" {
		t.Fatalf("symbolIndex = %v, want %q", got, "workspace")
	}
	if got := result["defaultDevelopmentEnvironment"]; got != "Delphi" {
		t.Fatalf("defaultDevelopmentEnvironment = %v, want %q", got, "Delphi")
	}
}

func TestRequireStringArgAllowEmpty(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    string
		wantErr bool
	}{
		{
			name:    "allows empty string",
			args:    map[string]any{"insertString": ""},
			key:     "insertString",
			want:    "",
			wantErr: false,
		},
		{
			name:    "rejects missing key",
			args:    map[string]any{},
			key:     "insertString",
			wantErr: true,
		},
		{
			name:    "rejects non-string value",
			args:    map[string]any{"insertString": 42},
			key:     "insertString",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requireStringArgAllowEmpty(tt.args, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("requireStringArgAllowEmpty() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("requireStringArgAllowEmpty() = %q, want %q", got, tt.want)
			}
		})
	}
}
