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
}`)

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
	if index.DefaultTarget != "~/.config/opencode/opencode.json" {
		t.Fatalf("expected defaultTarget preserved, got %q", index.DefaultTarget)
	}
}

func TestSettingIndexPreservesUnknownFieldsOnMarshal(t *testing.T) {
	index := SettingIndex{
		Description:  "perm note",
		DefaultTarget: "~/.config/opencode/opencode.json",
		Settings: map[string]SettingEntry{
			"GPT.json": {
				DisplayName: "GPT",
				Description: "keep me",
				Target:      "~/.config/opencode/special.json",
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
	if !bytes.Contains(data, []byte(`"templateHint":true`)) {
		t.Fatalf("expected extra field in output, got %s", data)
	}
	if !bytes.Contains(data, []byte(`"custom":{"x":1}`)) {
		t.Fatalf("expected nested extra field in output, got %s", data)
	}
}

func TestParseModeIndexRetainsMissingMarkersAndUnknownFields(t *testing.T) {
	input := []byte(`{
  "Max": {
    "displayName": "Max",
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
