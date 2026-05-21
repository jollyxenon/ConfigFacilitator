package index

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestParseSettingIndexStripsCommentsAndPreservesDescription(t *testing.T) {
	input := []byte(`{
  // comment
  "description": "perm note",
  "defaultTarget": "~/.config/opencode/opencode.json",
  "settings": {
    "GPT.json": {
      // disposable
      "displayName": "GPT",
      "description": "keep me"
    }
  }
}

/*
Example block that should be ignored during parsing.
*/`)

	index, err := ParseSettingIndex(input)
	if err != nil {
		t.Fatalf("ParseSettingIndex returned error: %v", err)
	}

	if index.Description != "perm note" {
		t.Fatalf("expected description to survive parsing, got %q", index.Description)
	}
	if index.Settings["GPT.json"].Description != "keep me" {
		t.Fatalf("expected nested description to survive parsing, got %q", index.Settings["GPT.json"].Description)
	}
	if index.Settings["GPT.json"].Aliases == nil {
		t.Fatalf("expected aliases normalized to empty slice")
	}
	if index.DefaultTarget != "~/.config/opencode/opencode.json" {
		t.Fatalf("expected defaultTarget preserved, got %q", index.DefaultTarget)
	}
}

func TestParseProjectAndColumnIndexesPreserveAdditionalIdentityShapedFields(t *testing.T) {
	projectInput := []byte(`{
  "OpenCode": {
    "folderName": "OpenCode",
    "displayName": "Open Code",
    "aliases": ["oc"]
  }
}`)
	projectIndex, err := ParseProjectIndex(projectInput)
	if err != nil {
		t.Fatalf("ParseProjectIndex returned error: %v", err)
	}
	if got := string(projectIndex.Projects["OpenCode"].Extra["folderName"]); got != `"OpenCode"` {
		t.Fatalf("expected folderName preserved in extra fields, got %q", got)
	}

	columnInput := []byte(`{
  "skills-dir": {
	    "warehouseName": "skills-dir",
	    "displayName": "Skills",
    "aliases": ["skills"],
    "custom": true
  }
}`)
	columnIndex, err := ParseColumnIndex(columnInput)
	if err != nil {
		t.Fatalf("ParseColumnIndex returned error: %v", err)
	}
	if got := string(columnIndex.Columns["skills-dir"].Extra["warehouseName"]); got != `"skills-dir"` {
		t.Fatalf("expected warehouseName preserved in extra fields, got %q", got)
	}
}

func TestSettingIndexPreservesUnknownFieldsOnMarshal(t *testing.T) {
	index := SettingIndex{
		Description:   "perm note",
		DefaultTarget: "~/.config/opencode/opencode.json",
		Settings: map[string]SettingEntry{
			"GPT.json": {
				DisplayName:   "GPT",
				Aliases:       []string{"gpt"},
				Description:   "keep me",
				Target:        "~/.config/opencode/special.json",
				Extra: map[string]json.RawMessage{
					"custom": json.RawMessage(`{"x":1}`),
				},
			},
		},
		Extra: map[string]json.RawMessage{
			"templateHint": json.RawMessage(`true`),
		},
	}

	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	if !bytes.Contains(data, []byte(`"description":"perm note"`)) {
		t.Fatalf("expected description in output, got %s", data)
	}
	if bytes.Contains(data, []byte(`"id":"gpt-setting"`)) {
		t.Fatalf("did not expect persisted id in output, got %s", data)
	}
	if bytes.Contains(data, []byte(`"warehouseName":`)) {
		t.Fatalf("did not expect warehouseName in output, got %s", data)
	}
	if !bytes.Contains(data, []byte(`"aliases":["gpt"]`)) {
		t.Fatalf("expected aliases in output, got %s", data)
	}
	if !bytes.Contains(data, []byte(`"templateHint":true`)) {
		t.Fatalf("expected extra field in output, got %s", data)
	}
	if !bytes.Contains(data, []byte(`"custom":{"x":1}`)) {
		t.Fatalf("expected nested extra field in output, got %s", data)
	}
}

func TestMarshalProjectEntryEmitsEmptyAliasesAndOmitsID(t *testing.T) {
	data, err := json.Marshal(ProjectEntry{DisplayName: "OpenCode"})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if bytes.Contains(data, []byte(`"id":`)) {
		t.Fatalf("did not expect id in output, got %s", data)
	}
	if !bytes.Contains(data, []byte(`"aliases":[]`)) {
		t.Fatalf("expected explicit empty aliases, got %s", data)
	}
}

func TestParseProjectIndexRejectsExplicitIDField(t *testing.T) {
	_, err := ParseProjectIndex([]byte(`{
  "OpenCode": {
    "id": "open-code"
  }
}`))
	if err == nil {
		t.Fatalf("expected explicit id field to be rejected")
	}
}

func TestParseProjectIndexPreservesDivergentAdditionalIdentityField(t *testing.T) {
	index, err := ParseProjectIndex([]byte(`{
  "OpenCode": {
    "warehouseName": "OtherName"
  }
}`))
	if err != nil {
		t.Fatalf("ParseProjectIndex returned error: %v", err)
	}
	if got := string(index.Projects["OpenCode"].Extra["warehouseName"]); got != `"OtherName"` {
		t.Fatalf("expected divergent warehouseName preserved in extra fields, got %q", got)
	}
}

func TestParseModeIndexPreservesAdditionalIdentityField(t *testing.T) {
	input := []byte(`{
  "Max": {
	    "warehouseName": "Max",
	    "displayName": "Max",
    "aliases": ["m"],
    "description": "mode note",
    "columns": {
      "Skills": {
        "settings": ["Skill-A"],
        "strategy": "incremental",
        "missing": true
      }
    }
  },
  "orphanedMode": {
    "missing": true,
    "description": "orphan"
  }
}`)

	index, err := ParseModeIndex(input)
	if err != nil {
		t.Fatalf("ParseModeIndex returned error: %v", err)
	}
	if got := string(index.Modes["Max"].Extra["warehouseName"]); got != `"Max"` {
		t.Fatalf("expected warehouseName preserved in extra fields, got %q", got)
	}
}

func TestParseModeIndexRetainsMissingMarkersAndUnknownFieldsWithoutLegacyIdentity(t *testing.T) {
	input := []byte(`{
  "Max": {
    "displayName": "Max",
    "aliases": ["m"],
    "description": "mode note",
    "columns": {
      "Skills": {
        "settings": ["Skill-A"],
        "strategy": "incremental",
        "missing": true
      }
    }
  },
  "orphanedMode": {
    "missing": true,
    "description": "orphan"
  }
}`)

	index, err := ParseModeIndex(input)
	if err != nil {
		t.Fatalf("ParseModeIndex returned error: %v", err)
	}

	if index.Modes["Max"].Description != "mode note" {
		t.Fatalf("expected mode description preserved, got %q", index.Modes["Max"].Description)
	}
	orphanedMode, ok := index.Modes["orphanedMode"]
	if !ok {
		t.Fatalf("expected orphaned mode entry to remain present")
	}
	if got := orphanedMode.Extra["missing"]; len(got) == 0 {
		t.Fatalf("expected orphaned mode to retain its missing marker")
	}
	if got := index.Modes["Max"].Columns["Skills"].Extra["missing"]; len(got) == 0 {
		t.Fatalf("expected unknown column field to be retained")
	}
}
