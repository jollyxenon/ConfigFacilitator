package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/xenon/ConfigFacilitator/internal/jsonc"
)

// ProjectIndex stores project-level metadata keyed by folder name.
type ProjectIndex struct {
	Projects map[string]ProjectEntry
	Extra    map[string]json.RawMessage
}

// ProjectEntry stores a single project record.
type ProjectEntry struct {
	FolderName string
	DisplayName string
	Aliases     []string
	Description string
	Extra       map[string]json.RawMessage
}

// ColumnIndex stores column-level metadata keyed by folder name.
type ColumnIndex struct {
	Columns map[string]ColumnEntry
	Extra   map[string]json.RawMessage
}

// ColumnEntry stores a single column record.
type ColumnEntry struct {
	FolderName string
	DisplayName string
	Description string
	Extra       map[string]json.RawMessage
}

// SettingIndex stores column-local setting metadata and targets.
type SettingIndex struct {
	Description  string
	DefaultTarget string
	Settings      map[string]SettingEntry
	Extra         map[string]json.RawMessage
}

// SettingEntry stores a single setting record.
type SettingEntry struct {
	DisplayName string
	Description string
	Target      string
	Extra       map[string]json.RawMessage
}

// ModeIndex stores mode metadata keyed by mode name.
type ModeIndex struct {
	Modes map[string]ModeEntry
	Extra map[string]json.RawMessage
}

// ModeEntry stores one mode and its column mappings.
type ModeEntry struct {
	DisplayName string
	Description string
	Columns     map[string]ModeColumnSelection
	Extra       map[string]json.RawMessage
}

// ModeColumnSelection stores one column selection inside a mode.
type ModeColumnSelection struct {
	Settings []string
	Strategy string
	Extra    map[string]json.RawMessage
}

// ParseProjectIndex parses JSONC project metadata into a ProjectIndex.
func ParseProjectIndex(src []byte) (ProjectIndex, error) {
	var raw map[string]json.RawMessage
	if err := jsonc.Unmarshal(src, &raw); err != nil {
		return ProjectIndex{}, err
	}
	return parseProjectIndex(raw)
}

// ParseColumnIndex parses JSONC column metadata into a ColumnIndex.
func ParseColumnIndex(src []byte) (ColumnIndex, error) {
	var raw map[string]json.RawMessage
	if err := jsonc.Unmarshal(src, &raw); err != nil {
		return ColumnIndex{}, err
	}
	return parseColumnIndex(raw)
}

// ParseSettingIndex parses JSONC setting metadata into a SettingIndex.
func ParseSettingIndex(src []byte) (SettingIndex, error) {
	var raw map[string]json.RawMessage
	if err := jsonc.Unmarshal(src, &raw); err != nil {
		return SettingIndex{}, err
	}
	return parseSettingIndex(raw)
}

// ParseModeIndex parses JSONC mode metadata into a ModeIndex.
func ParseModeIndex(src []byte) (ModeIndex, error) {
	var raw map[string]json.RawMessage
	if err := jsonc.Unmarshal(src, &raw); err != nil {
		return ModeIndex{}, err
	}
	return parseModeIndex(raw)
}

// MarshalJSON emits the stable JSON representation for a ProjectIndex.
func (index ProjectIndex) MarshalJSON() ([]byte, error) { return marshalProjectIndex(index) }

// MarshalJSON emits the stable JSON representation for a ColumnIndex.
func (index ColumnIndex) MarshalJSON() ([]byte, error) { return marshalColumnIndex(index) }

// MarshalJSON emits the stable JSON representation for a SettingIndex.
func (index SettingIndex) MarshalJSON() ([]byte, error) { return marshalSettingIndex(index) }

// MarshalJSON emits the stable JSON representation for a ModeIndex.
func (index ModeIndex) MarshalJSON() ([]byte, error) { return marshalModeIndex(index) }

func parseProjectIndex(raw map[string]json.RawMessage) (ProjectIndex, error) {
	index := ProjectIndex{Projects: map[string]ProjectEntry{}, Extra: map[string]json.RawMessage{}}
	for key, value := range raw {
		entry, err := parseProjectEntry(key, value)
		if err != nil {
			return ProjectIndex{}, err
		}
		if entry.IsEmpty() {
			index.Extra[key] = value
			continue
		}
		index.Projects[key] = entry
	}
	return index, nil
}

func parseColumnIndex(raw map[string]json.RawMessage) (ColumnIndex, error) {
	index := ColumnIndex{Columns: map[string]ColumnEntry{}, Extra: map[string]json.RawMessage{}}
	for key, value := range raw {
		entry, err := parseColumnEntry(key, value)
		if err != nil {
			return ColumnIndex{}, err
		}
		if entry.IsEmpty() {
			index.Extra[key] = value
			continue
		}
		index.Columns[key] = entry
	}
	return index, nil
}

func parseSettingIndex(raw map[string]json.RawMessage) (SettingIndex, error) {
	index := SettingIndex{Settings: map[string]SettingEntry{}, Extra: map[string]json.RawMessage{}}
	for key, value := range raw {
		switch key {
		case "description":
			if err := json.Unmarshal(value, &index.Description); err != nil { return SettingIndex{}, err }
		case "defaultTarget":
			if err := json.Unmarshal(value, &index.DefaultTarget); err != nil { return SettingIndex{}, err }
		case "settings":
			var settings map[string]json.RawMessage
			if err := json.Unmarshal(value, &settings); err != nil { return SettingIndex{}, err }
			for settingKey, settingValue := range settings {
				entry, err := parseSettingEntry(settingKey, settingValue)
				if err != nil { return SettingIndex{}, err }
				index.Settings[settingKey] = entry
			}
		default:
			index.Extra[key] = value
		}
	}
	return index, nil
}

func parseModeIndex(raw map[string]json.RawMessage) (ModeIndex, error) {
	index := ModeIndex{Modes: map[string]ModeEntry{}, Extra: map[string]json.RawMessage{}}
	for key, value := range raw {
		entry, err := parseModeEntry(key, value)
		if err != nil { return ModeIndex{}, err }
		if entry.IsEmpty() {
			index.Extra[key] = value
			continue
		}
		index.Modes[key] = entry
	}
	return index, nil
}

func parseProjectEntry(folderName string, raw json.RawMessage) (ProjectEntry, error) {
	entry := ProjectEntry{FolderName: folderName, Extra: map[string]json.RawMessage{}}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) { return entry, nil }
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil { return ProjectEntry{}, err }
	for key, value := range data {
		switch key {
		case "folderName":
			if err := json.Unmarshal(value, &entry.FolderName); err != nil { return ProjectEntry{}, err }
		case "displayName":
			if err := json.Unmarshal(value, &entry.DisplayName); err != nil { return ProjectEntry{}, err }
		case "aliases":
			if err := json.Unmarshal(value, &entry.Aliases); err != nil { return ProjectEntry{}, err }
		case "description":
			if err := json.Unmarshal(value, &entry.Description); err != nil { return ProjectEntry{}, err }
		default:
			entry.Extra[key] = value
		}
	}
	return entry, nil
}

func parseColumnEntry(folderName string, raw json.RawMessage) (ColumnEntry, error) {
	entry := ColumnEntry{FolderName: folderName, Extra: map[string]json.RawMessage{}}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) { return entry, nil }
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil { return ColumnEntry{}, err }
	for key, value := range data {
		switch key {
		case "folderName":
			if err := json.Unmarshal(value, &entry.FolderName); err != nil { return ColumnEntry{}, err }
		case "displayName":
			if err := json.Unmarshal(value, &entry.DisplayName); err != nil { return ColumnEntry{}, err }
		case "description":
			if err := json.Unmarshal(value, &entry.Description); err != nil { return ColumnEntry{}, err }
		default:
			entry.Extra[key] = value
		}
	}
	return entry, nil
}

func parseSettingEntry(name string, raw json.RawMessage) (SettingEntry, error) {
	entry := SettingEntry{DisplayName: name, Extra: map[string]json.RawMessage{}}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) { return entry, nil }
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil { return SettingEntry{}, err }
	for key, value := range data {
		switch key {
		case "displayName":
			if err := json.Unmarshal(value, &entry.DisplayName); err != nil { return SettingEntry{}, err }
		case "description":
			if err := json.Unmarshal(value, &entry.Description); err != nil { return SettingEntry{}, err }
		case "target":
			if err := json.Unmarshal(value, &entry.Target); err != nil { return SettingEntry{}, err }
		default:
			entry.Extra[key] = value
		}
	}
	return entry, nil
}

func parseModeEntry(name string, raw json.RawMessage) (ModeEntry, error) {
	entry := ModeEntry{DisplayName: name, Columns: map[string]ModeColumnSelection{}, Extra: map[string]json.RawMessage{}}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) { return entry, nil }
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil { return ModeEntry{}, err }
	for key, value := range data {
		switch key {
		case "displayName":
			if err := json.Unmarshal(value, &entry.DisplayName); err != nil { return ModeEntry{}, err }
		case "description":
			if err := json.Unmarshal(value, &entry.Description); err != nil { return ModeEntry{}, err }
		case "columns":
			var cols map[string]json.RawMessage
			if err := json.Unmarshal(value, &cols); err != nil { return ModeEntry{}, err }
			for columnName, columnValue := range cols {
				selection, err := parseModeColumnSelection(columnName, columnValue)
				if err != nil { return ModeEntry{}, err }
				entry.Columns[columnName] = selection
			}
		default:
			entry.Extra[key] = value
		}
	}
	return entry, nil
}

func parseModeColumnSelection(name string, raw json.RawMessage) (ModeColumnSelection, error) {
	selection := ModeColumnSelection{Extra: map[string]json.RawMessage{}}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) { return selection, nil }
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil { return ModeColumnSelection{}, err }
	for key, value := range data {
		switch key {
		case "settings":
			if err := json.Unmarshal(value, &selection.Settings); err != nil { return ModeColumnSelection{}, err }
		case "strategy":
			if err := json.Unmarshal(value, &selection.Strategy); err != nil { return ModeColumnSelection{}, err }
		default:
			selection.Extra[key] = value
		}
	}
	_ = name
	return selection, nil
}

func marshalProjectIndex(index ProjectIndex) ([]byte, error) {
	data := map[string]any{}
	for key, entry := range index.Projects {
		data[key] = entry
	}
	mergeRaw(data, index.Extra)
	return marshalObject(data)
}

func marshalColumnIndex(index ColumnIndex) ([]byte, error) {
	data := map[string]any{}
	for key, entry := range index.Columns {
		data[key] = entry
	}
	mergeRaw(data, index.Extra)
	return marshalObject(data)
}

func marshalSettingIndex(index SettingIndex) ([]byte, error) {
	data := map[string]any{}
	if index.Description != "" {
		data["description"] = index.Description
	}
	if index.DefaultTarget != "" {
		data["defaultTarget"] = index.DefaultTarget
	}
	settings := map[string]any{}
	for key, entry := range index.Settings {
		settings[key] = entry
	}
	if len(settings) > 0 {
		data["settings"] = settings
	}
	mergeRaw(data, index.Extra)
	return marshalObject(data)
}

func marshalModeIndex(index ModeIndex) ([]byte, error) {
	data := map[string]any{}
	for key, entry := range index.Modes {
		data[key] = entry
	}
	mergeRaw(data, index.Extra)
	return marshalObject(data)
}

func (entry ProjectEntry) MarshalJSON() ([]byte, error) { return marshalProjectEntry(entry) }
func (entry ColumnEntry) MarshalJSON() ([]byte, error) { return marshalColumnEntry(entry) }
func (entry SettingEntry) MarshalJSON() ([]byte, error) { return marshalSettingEntry(entry) }
func (entry ModeEntry) MarshalJSON() ([]byte, error) { return marshalModeEntry(entry) }
func (entry ModeColumnSelection) MarshalJSON() ([]byte, error) { return marshalModeColumnSelection(entry) }

func (entry ProjectEntry) IsEmpty() bool { return entry.FolderName == "" && entry.DisplayName == "" && len(entry.Aliases) == 0 && entry.Description == "" && len(entry.Extra) == 0 }
func (entry ColumnEntry) IsEmpty() bool { return entry.FolderName == "" && entry.DisplayName == "" && entry.Description == "" && len(entry.Extra) == 0 }
func (entry ModeEntry) IsEmpty() bool { return entry.DisplayName == "" && entry.Description == "" && len(entry.Columns) == 0 && len(entry.Extra) == 0 }

func marshalProjectEntry(entry ProjectEntry) ([]byte, error) {
	data := map[string]any{}
	if entry.FolderName != "" { data["folderName"] = entry.FolderName }
	if entry.DisplayName != "" { data["displayName"] = entry.DisplayName }
	if len(entry.Aliases) > 0 { data["aliases"] = entry.Aliases }
	if entry.Description != "" { data["description"] = entry.Description }
	mergeRaw(data, entry.Extra)
	return marshalObject(data)
}

func marshalColumnEntry(entry ColumnEntry) ([]byte, error) {
	data := map[string]any{}
	if entry.FolderName != "" { data["folderName"] = entry.FolderName }
	if entry.DisplayName != "" { data["displayName"] = entry.DisplayName }
	if entry.Description != "" { data["description"] = entry.Description }
	mergeRaw(data, entry.Extra)
	return marshalObject(data)
}

func marshalSettingEntry(entry SettingEntry) ([]byte, error) {
	data := map[string]any{}
	if entry.DisplayName != "" { data["displayName"] = entry.DisplayName }
	if entry.Description != "" { data["description"] = entry.Description }
	if entry.Target != "" { data["target"] = entry.Target }
	mergeRaw(data, entry.Extra)
	return marshalObject(data)
}

func marshalModeEntry(entry ModeEntry) ([]byte, error) {
	data := map[string]any{}
	if entry.DisplayName != "" { data["displayName"] = entry.DisplayName }
	if entry.Description != "" { data["description"] = entry.Description }
	columns := map[string]any{}
	for key, selection := range entry.Columns { columns[key] = selection }
	if len(columns) > 0 { data["columns"] = columns }
	mergeRaw(data, entry.Extra)
	return marshalObject(data)
}

func marshalModeColumnSelection(entry ModeColumnSelection) ([]byte, error) {
	data := map[string]any{}
	if len(entry.Settings) > 0 { data["settings"] = entry.Settings }
	if entry.Strategy != "" { data["strategy"] = entry.Strategy }
	mergeRaw(data, entry.Extra)
	return marshalObject(data)
}

func mergeRaw(dst map[string]any, raw map[string]json.RawMessage) {
	for key, value := range raw {
		if _, exists := dst[key]; exists {
			continue
		}
		dst[key] = json.RawMessage(value)
	}
}

func marshalObject(data map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]any, len(data))
	for _, key := range keys {
		ordered[key] = data[key]
	}
	if ordered == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(ordered)
}

func validateNoNilMaps(index any) error {
	switch v := index.(type) {
	case ProjectIndex:
		if v.Projects == nil { return fmt.Errorf("project index projects map is nil") }
	case ColumnIndex:
		if v.Columns == nil { return fmt.Errorf("column index columns map is nil") }
	case SettingIndex:
		if v.Settings == nil { return fmt.Errorf("setting index settings map is nil") }
	case ModeIndex:
		if v.Modes == nil { return fmt.Errorf("mode index modes map is nil") }
	}
	return nil
}
