package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"unicode/utf8"

	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// resolveSetting loads the Setting model for one content command.
func resolveSetting(rootPath, projectName, columnName, settingName string) (warehouse.Project, warehouse.Column, warehouse.Setting, error) {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	project, err := loaded.ResolveProject(projectName)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	column, err := project.ResolveColumn(columnName)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	setting, err := column.ResolveSetting(settingName)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	return project, column, setting, nil
}

// settingKindFor resolves the Setting kind for content operations.
func settingKindFor(setting warehouse.Setting) content.Kind {
	if setting.IsDir {
		return content.KindDirectory
	}
	return content.KindFile
}

func (handler *Handler) settingContentList(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	_, column, setting, err := resolveSetting(rootPath, payload.Project, payload.Column, payload.Setting)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	entries, err := content.List(setting.Path, settingKindFor(setting))
	if err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return http.StatusOK, commandResult{Message: fmt.Sprintf("%d entries", len(entries)), Details: map[string]any{"column": column.Name, "setting": setting.Name, "entries": entries}}, nil
}

func (handler *Handler) settingContentRead(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	_, _, setting, err := resolveSetting(rootPath, payload.Project, payload.Column, payload.Setting)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	var relative *string
	if payload.Path != "" {
		relative = &payload.Path
	}
	data, err := content.Read(setting.Path, settingKindFor(setting), relative)
	if err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	encoding := "utf8"
	text := string(data)
	if !validUTF8(data) {
		encoding = "base64"
		text = base64.StdEncoding.EncodeToString(data)
	}
	return http.StatusOK, commandResult{Message: "", Details: map[string]any{
		"column":   payload.Column,
		"setting":  payload.Setting,
		"path":     nullableString(relative),
		"content":  text,
		"encoding": encoding,
		"bytes":    len(data),
	}}, nil
}

func (handler *Handler) settingContentWrite(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	_, _, setting, err := resolveSetting(rootPath, payload.Project, payload.Column, payload.Setting)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	var data []byte
	switch payload.Encoding {
	case "", "utf8":
		data = []byte(payload.Content)
	case "base64":
		data, err = base64.StdEncoding.DecodeString(payload.Content)
		if err != nil {
			return http.StatusBadRequest, commandResult{}, &errorBody{Code: "invalid_base64", Message: "invalid base64 content"}
		}
	default:
		return http.StatusBadRequest, commandResult{}, &errorBody{Code: "unsupported_encoding", Message: fmt.Sprintf("unsupported encoding %q", payload.Encoding)}
	}
	var relative *string
	if payload.Path != "" {
		relative = &payload.Path
	}
	if err := content.Write(repository.New(rootPath), setting.Path, settingKindFor(setting), relative, content.Source{Mode: content.SourceBytes, Bytes: data}); err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	return http.StatusOK, commandResult{Message: "Updated Setting content"}, nil
}

func (handler *Handler) settingContentMkdir(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	_, _, setting, err := resolveSetting(rootPath, payload.Project, payload.Column, payload.Setting)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	if err := content.Mkdir(repository.New(rootPath), setting.Path, settingKindFor(setting), payload.Path); err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	return http.StatusOK, commandResult{Message: "Created content directory"}, nil
}

func (handler *Handler) settingContentMove(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	_, _, setting, err := resolveSetting(rootPath, payload.Project, payload.Column, payload.Setting)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	if err := content.Move(repository.New(rootPath), setting.Path, settingKindFor(setting), payload.OldPath, payload.Path); err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	return http.StatusOK, commandResult{Message: "Moved content"}, nil
}

func (handler *Handler) settingContentDelete(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	_, _, setting, err := resolveSetting(rootPath, payload.Project, payload.Column, payload.Setting)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	if err := content.Delete(repository.New(rootPath), setting.Path, settingKindFor(setting), payload.Path, payload.Yes); err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	return http.StatusOK, commandResult{Message: "Deleted content"}, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func validUTF8(data []byte) bool {
	return utf8.Valid(data)
}
