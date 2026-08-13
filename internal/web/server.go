package web

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/syncer"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
)

// WebDependencies carries process context needed by the local HTTP server.
type WebDependencies struct {
	HomeDir         string
	Environment     map[string]string
	OperatingSystem string
	Stdout          io.Writer
}

// Handler serves the embedded UI and the local snapshot/command API for one
// effective warehouse root. The root changes only through the root command.
type Handler struct {
	dependencies WebDependencies
	rootPath     string
	mu           sync.Mutex
}

// NewHandler builds one handler bound to the current effective warehouse root.
func NewHandler(dependencies WebDependencies) (*Handler, error) {
	rootPath, err := warehouse.EffectiveWarehouseRootFor(dependencies.HomeDir)
	if err != nil {
		return nil, err
	}
	return &Handler{dependencies: dependencies, rootPath: rootPath}, nil
}

// Root returns the current effective warehouse root.
func (handler *Handler) Root() string {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	return handler.rootPath
}

// ServeHTTP routes embedded assets and the local API.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	switch {
	case path == "/api/snapshot" && request.Method == http.MethodGet:
		handler.serveSnapshot(writer)
	case path == "/api/command" && request.Method == http.MethodPost:
		handler.serveCommand(writer, request)
	case path == "/api/preview" && request.Method == http.MethodPost:
		handler.servePreview(writer, request)
	case strings.HasPrefix(path, "/api/"):
		writeError(writer, http.StatusNotFound, &errorBody{Code: "route_not_found", Message: "unknown API route"})
	default:
		handler.serveStatic(writer, request)
	}
}

// serveStatic serves the embedded frontend with index fallback.
func (handler *Handler) serveStatic(writer http.ResponseWriter, request *http.Request) {
	name := strings.TrimPrefix(request.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(staticFS, "static/"+name)
	if err != nil {
		name = "index.html"
		data, err = fs.ReadFile(staticFS, "static/"+name)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
	}
	contentType := "text/plain"
	switch {
	case strings.HasSuffix(name, ".html"):
		contentType = "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		contentType = "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		contentType = "application/javascript; charset=utf-8"
	}
	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(data)
}

// ================= snapshot =================

type snapshotSetting struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	Kind        string   `json:"kind"`
	TargetDir   []string `json:"targetDir"`
	TargetName  []string `json:"targetName"`
}

type snapshotColumn struct {
	Name              string                     `json:"name"`
	DisplayName       string                     `json:"displayName"`
	Description       string                     `json:"description"`
	Aliases           []string                   `json:"aliases"`
	TargetNumber      int                        `json:"targetNumber"`
	DefaultTargetDir  []string                   `json:"defaultTargetDir"`
	DefaultTargetName []string                   `json:"defaultTargetName"`
	Settings          map[string]snapshotSetting `json:"settings"`
}

type snapshotModeColumn struct {
	Strategy string   `json:"strategy"`
	Settings []string `json:"settings"`
}

type snapshotMode struct {
	Name        string                        `json:"name"`
	DisplayName string                        `json:"displayName"`
	Description string                        `json:"description"`
	Aliases     []string                      `json:"aliases"`
	Columns     map[string]snapshotModeColumn `json:"columns"`
}

type projectSnapshot struct {
	Name        string                    `json:"name"`
	DisplayName string                    `json:"displayName"`
	Aliases     []string                  `json:"aliases"`
	Available   bool                      `json:"available"`
	Error       *errorBody                `json:"error,omitempty"`
	Columns     map[string]snapshotColumn `json:"columns"`
	Modes       map[string]snapshotMode   `json:"modes"`
	Current     *repository.CurrentState  `json:"current,omitempty"`
}

type snapshot struct {
	Root         string                       `json:"root"`
	Revision     string                       `json:"revision"`
	Projects     map[string]projectSnapshot   `json:"projects"`
	Targets      map[string]string            `json:"targets"`
	Transactions []repository.TransactionInfo `json:"transactions,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type responseEnvelope struct {
	OK    bool       `json:"ok"`
	Data  any        `json:"data,omitempty"`
	Error *errorBody `json:"error,omitempty"`
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, body *errorBody) {
	writeJSON(writer, status, responseEnvelope{OK: false, Error: body})
}

func (handler *Handler) serveSnapshot(writer http.ResponseWriter) {
	handler.mu.Lock()
	rootPath := handler.rootPath
	handler.mu.Unlock()
	value, err := handler.buildSnapshot(rootPath)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, &errorBody{Code: "snapshot_failed", Message: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, responseEnvelope{OK: true, Data: value})
}

func (handler *Handler) buildSnapshot(rootPath string) (snapshot, error) {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return snapshot{}, err
	}
	revision, err := ComputeRevision(rootPath)
	if err != nil {
		return snapshot{}, err
	}
	result := snapshot{Root: rootPath, Revision: revision, Projects: map[string]projectSnapshot{}, Targets: map[string]string{}}
	for _, projectName := range sortedKeys(loaded.Projects) {
		project := loaded.Projects[projectName]
		item := projectSnapshot{
			Name:        project.Name,
			DisplayName: project.Metadata.DisplayName,
			Aliases:     project.Metadata.Aliases,
			Available:   true,
			Columns:     map[string]snapshotColumn{},
			Modes:       map[string]snapshotMode{},
		}
		state, stateErr := repository.New(rootPath).LoadCurrentState(project.Name)
		if stateErr != nil {
			item.Available = false
			item.Error = &errorBody{Code: "current_state_unsupported", Message: stateErr.Error()}
		} else {
			item.Current = &state
		}
		for _, columnName := range sortedKeys(project.Columns) {
			column := project.Columns[columnName]
			col := snapshotColumn{
				Name:              column.Name,
				DisplayName:       column.Metadata.DisplayName,
				Description:       column.Metadata.Description,
				Aliases:           column.Metadata.Aliases,
				TargetNumber:      column.SettingIndex.TargetNumber,
				DefaultTargetDir:  column.SettingIndex.DefaultTargetDir,
				DefaultTargetName: column.SettingIndex.DefaultTargetName,
				Settings:          map[string]snapshotSetting{},
			}
			for _, settingName := range sortedKeys(column.Settings) {
				setting := column.Settings[settingName]
				col.Settings[settingName] = snapshotSetting{
					Name:        setting.Name,
					DisplayName: setting.Metadata.DisplayName,
					Description: setting.Metadata.Description,
					Aliases:     setting.Metadata.Aliases,
					Kind:        settingKind(setting),
					TargetDir:   setting.Metadata.TargetDir,
					TargetName:  setting.Metadata.TargetName,
				}
			}
			item.Columns[columnName] = col
		}
		for _, modeName := range sortedKeys(project.Modes) {
			mode := project.Modes[modeName]
			modeItem := snapshotMode{
				Name:        mode.Name,
				DisplayName: mode.Metadata.DisplayName,
				Description: mode.Metadata.Description,
				Aliases:     mode.Metadata.Aliases,
				Columns:     map[string]snapshotModeColumn{},
			}
			for reference, selection := range mode.Metadata.Columns {
				modeItem.Columns[reference] = snapshotModeColumn{Strategy: selection.Strategy, Settings: append([]string{}, selection.Settings...)}
			}
			item.Modes[modeName] = modeItem
		}
		for _, column := range project.Columns {
			for _, setting := range column.Settings {
				if setting.Missing || column.SettingIndex.TargetNumber == 0 {
					continue
				}
				planned, planErr := planner.PlanColumns(project, map[string]index.ModeColumnSelection{column.Name: {Strategy: "cover", Settings: []string{setting.Name}}}, nil, planOptions(handler.dependencies))
				if planErr != nil {
					continue
				}
				for _, mapping := range planned {
					if _, exists := result.Targets[mapping.Target]; exists {
						continue
					}
					result.Targets[mapping.Target] = targetState(mapping)
				}
			}
		}
		result.Projects[projectName] = item
	}
	return result, nil
}

func settingKind(setting warehouse.Setting) string {
	if setting.Missing || !setting.Exists {
		return "missing"
	}
	if setting.IsDir {
		return "directory"
	}
	return "file"
}

func targetState(mapping repository.Mapping) string {
	ownership, err := linker.New().InspectOwnership(mapping)
	if err != nil {
		return "unmanaged"
	}
	switch ownership {
	case linker.OwnershipOwned:
		return "ok"
	case linker.OwnershipAbsent:
		return "free"
	default:
		return "occupied"
	}
}

func planOptions(dependencies WebDependencies) planner.PlanOptions {
	return planner.PlanOptions{HomeDir: dependencies.HomeDir, Env: dependencies.Environment, OS: dependencies.OperatingSystem}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ================= revision =================

// ComputeRevision returns a stable fingerprint of every persisted warehouse
// file (indexes, runtime state, and the root bootstrap), excluding sessions,
// transactions, Setting source content, and external target files.
func ComputeRevision(rootPath string) (string, error) {
	hash := sha256.New()
	entries := []string{}
	addFile := func(path string, info os.FileInfo) {
		relative, err := filepath.Rel(rootPath, path)
		if err != nil {
			return
		}
		entries = append(entries, fmt.Sprintf("%s\x00%d\x00%d", filepath.ToSlash(relative), info.Size(), info.ModTime().UnixNano()))
	}
	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		switch {
		case name == "ProjectIndex.jsonc":
			addFile(path, info)
		case name == "ColumnIndex.jsonc":
			addFile(path, info)
		case name == "SettingIndex.jsonc":
			addFile(path, info)
		case name == "ModeIndex.jsonc":
			addFile(path, info)
		case name == "current_state.json":
			addFile(path, info)
		case name == "history.log":
			addFile(path, info)
		case name == ".cfgfc-root":
			addFile(path, info)
		}
		return nil
	})
	sort.Strings(entries)
	for _, entry := range entries {
		_, _ = io.WriteString(hash, entry)
		_, _ = io.WriteString(hash, "\n")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ================= command API =================

type commandRequest struct {
	Command      string                                `json:"command"`
	Revision     string                                `json:"revision"`
	Project      string                                `json:"project"`
	Column       string                                `json:"column"`
	Setting      string                                `json:"setting"`
	Mode         string                                `json:"mode"`
	Name         string                                `json:"name"`
	NewName      string                                `json:"newName"`
	DisplayName  string                                `json:"displayName"`
	Description  string                                `json:"description"`
	Aliases      []string                              `json:"aliases"`
	Path         string                                `json:"path"`
	OldPath      string                                `json:"oldPath"`
	Strategy     string                                `json:"strategy"`
	Settings     []string                              `json:"settings"`
	Columns      map[string]repository.ColumnSelection `json:"columns"`
	Kind         string                                `json:"kind"`
	Content      string                                `json:"content"`
	Encoding     string                                `json:"encoding"`
	Yes          bool                                  `json:"yes"`
	Cascade      bool                                  `json:"cascade"`
	ForceTargets bool                                  `json:"forceTargets"`
	All          bool                                  `json:"all"`
}

// registeredCommands lists every command accepted by the local API.
var registeredCommands = map[string]bool{
	"project.create": true, "project.delete": true,
	"column.delete":        true,
	"setting.delete":       true,
	"setting.content.list": true, "setting.content.read": true, "setting.content.write": true,
	"setting.content.mkdir": true, "setting.content.move": true, "setting.content.delete": true,
	"mode.replace": true, "mode.delete": true, "mode.preview": true,
	"current.replace": true, "current.column.set": true, "current.column.delete": true, "current.preview": true,
	"apply.mode": true, "apply.column": true,
	"refresh": true, "reset": true, "revert": true, "sync": true, "root": true,
}

func isRegisteredCommand(name string) bool { return registeredCommands[name] }

// readOnlyCommands lists commands that never require or mutate revision state.
var readOnlyCommands = map[string]bool{
	"setting.content.list": true,
	"setting.content.read": true,
	"mode.preview":         true,
	"current.preview":      true,
}

func (handler *Handler) serveCommand(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 4<<20))
	if err != nil {
		writeError(writer, http.StatusBadRequest, &errorBody{Code: "read_body", Message: err.Error()})
		return
	}
	var payload commandRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, &errorBody{Code: "invalid_json", Message: err.Error()})
		return
	}
	handler.mu.Lock()
	rootPath := handler.rootPath
	handler.mu.Unlock()

	if !isRegisteredCommand(payload.Command) {
		writeError(writer, http.StatusBadRequest, &errorBody{Code: "command_not_supported", Message: fmt.Sprintf("command %q is not supported", payload.Command)})
		return
	}
	readOnly := readOnlyCommands[payload.Command]
	if !readOnly && payload.Revision == "" {
		writeError(writer, http.StatusBadRequest, &errorBody{Code: "revision_required", Message: "mutating commands require a revision"})
		return
	}
	if !readOnly {
		current, err := ComputeRevision(rootPath)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, &errorBody{Code: "revision_failed", Message: err.Error()})
			return
		}
		if current != payload.Revision {
			writeError(writer, http.StatusConflict, &errorBody{Code: "revision_conflict", Message: "warehouse changed since the page snapshot"})
			return
		}
	}

	status, result, bodyErr := handler.executeCommand(rootPath, payload)
	if bodyErr != nil {
		writeError(writer, status, bodyErr)
		return
	}
	if readOnly {
		writeJSON(writer, status, responseEnvelope{OK: true, Data: result})
		return
	}
	snapshot, err := handler.buildSnapshot(rootPath)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, &errorBody{Code: "snapshot_failed", Message: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, responseEnvelope{OK: true, Data: map[string]any{"snapshot": snapshot, "message": result.Message}})
}

type commandResult struct {
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (handler *Handler) executeCommand(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	repo := repository.New(rootPath)
	options := planOptions(handler.dependencies)
	force := payload.ForceTargets
	switch payload.Command {
	case "project.create":
		if err := workflow.ProjectCreate(repo, payload.Name, payload.DisplayName, payload.Description, payload.Aliases); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Created project " + payload.Name}, nil
	case "project.delete":
		if err := workflow.ProjectDelete(repo, payload.Name, payload.Yes, payload.ForceTargets); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Deleted project " + payload.Name}, nil
	case "column.delete":
		if err := workflow.ColumnDelete(repo, payload.Project, payload.Column, payload.Yes, payload.Cascade, payload.ForceTargets); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Deleted column " + payload.Column}, nil
	case "setting.delete":
		if err := workflow.SettingDelete(repo, payload.Project, payload.Column, payload.Setting, payload.Yes, payload.Cascade, payload.ForceTargets); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Deleted setting " + payload.Setting}, nil
	case "mode.replace":
		if err := workflow.ReplaceMode(repo, payload.Project, payload.Mode, payload.Columns, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Replaced mode " + payload.Mode}, nil
	case "mode.delete":
		if err := workflow.DeleteModeWorkflow(repo, payload.Project, payload.Mode, payload.Yes, force); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Deleted mode " + payload.Mode}, nil
	case "current.replace":
		if err := workflow.ReplaceCurrent(repo, payload.Project, payload.Columns, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Applied current selection"}, nil
	case "current.column.set":
		if err := workflow.SetCurrentColumn(repo, payload.Project, payload.Column, payload.Strategy, payload.Settings, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Set current column " + payload.Column}, nil
	case "current.column.delete":
		if err := workflow.DeleteCurrentColumn(repo, payload.Project, payload.Column, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Deleted current column " + payload.Column}, nil
	case "apply.mode":
		if err := workflow.ApplyMode(repo, payload.Project, payload.Mode, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Applied mode " + payload.Mode}, nil
	case "apply.column":
		if err := workflow.ApplyColumn(repo, payload.Project, payload.Column, payload.Settings, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Applied column " + payload.Column}, nil
	case "refresh":
		if err := workflow.RefreshCurrent(repo, payload.Project, force, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Refreshed project " + payload.Project}, nil
	case "reset":
		if err := workflow.ResetCurrent(repo, payload.Project, force); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Reset project " + payload.Project}, nil
	case "revert":
		if err := workflow.RevertCurrent(repo, payload.Project, force); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Reverted project " + payload.Project}, nil
	case "sync":
		if payload.All {
			return handler.syncAll(rootPath, options)
		}
		if err := syncer.SyncProject(rootPath, payload.Project, options); err != nil {
			return classifyErr(err)
		}
		return http.StatusOK, commandResult{Message: "Synchronized project " + payload.Project}, nil
	case "root":
		newRoot, err := warehouse.SetEffectiveWarehouseRootFor(handler.dependencies.HomeDir, handler.dependencies.OperatingSystem, payload.Path, handler.dependencies.Environment)
		if err != nil {
			return http.StatusInternalServerError, commandResult{}, &errorBody{Code: "set_warehouse_root", Message: err.Error()}
		}
		handler.mu.Lock()
		handler.rootPath = newRoot
		handler.mu.Unlock()
		return http.StatusOK, commandResult{Message: "Warehouse root set to " + newRoot}, nil
	case "setting.content.list":
		return handler.settingContentList(rootPath, payload)
	case "setting.content.read":
		return handler.settingContentRead(rootPath, payload)
	case "setting.content.write":
		return handler.settingContentWrite(rootPath, payload)
	case "setting.content.mkdir":
		return handler.settingContentMkdir(rootPath, payload)
	case "setting.content.move":
		return handler.settingContentMove(rootPath, payload)
	case "setting.content.delete":
		return handler.settingContentDelete(rootPath, payload)
	case "mode.preview":
		return handler.modePreview(rootPath, payload)
	case "current.preview":
		return handler.currentPreview(rootPath, payload)
	default:
		return http.StatusBadRequest, commandResult{}, &errorBody{Code: "command_not_supported", Message: fmt.Sprintf("command %q is not supported", payload.Command)}
	}
}

// classifyErr maps workflow failures to HTTP status and envelope fields.
func classifyErr(err error) (int, commandResult, *errorBody) {
	class, code := workflow.Classify(err)
	message := err.Error()
	switch class {
	case workflow.ErrNotFound:
		return http.StatusNotFound, commandResult{}, &errorBody{Code: code, Message: message}
	case workflow.ErrConflict:
		return http.StatusConflict, commandResult{}, &errorBody{Code: code, Message: message}
	case workflow.ErrInvalid:
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: code, Message: message}
	case workflow.ErrRefused:
		return http.StatusConflict, commandResult{}, &errorBody{Code: code, Message: message}
	default:
		return http.StatusInternalServerError, commandResult{}, &errorBody{Code: code, Message: message}
	}
}

func (handler *Handler) syncAll(rootPath string, options planner.PlanOptions) (int, commandResult, *errorBody) {
	succeeded := []string{}
	failed := map[string]errorBody{}
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return http.StatusInternalServerError, commandResult{}, &errorBody{Code: "sync_failed", Message: err.Error()}
	}
	if err := syncer.SyncProjectIndexOnly(rootPath); err != nil {
		return http.StatusInternalServerError, commandResult{}, &errorBody{Code: "sync_failed", Message: err.Error()}
	}
	for _, projectName := range sortedKeys(loaded.Projects) {
		if err := syncer.SyncProject(rootPath, projectName, options); err != nil {
			failed[projectName] = errorBody{Code: "sync_failed", Message: err.Error()}
			continue
		}
		succeeded = append(succeeded, projectName)
	}
	status := http.StatusOK
	if len(failed) > 0 {
		status = http.StatusMultiStatus
	}
	return status, commandResult{Message: fmt.Sprintf("Synchronized %d project(s), %d failed", len(succeeded), len(failed)), Details: map[string]any{"succeeded": succeeded, "failed": failed}}, nil
}

// servePreview plans a draft selection without committing.
func (handler *Handler) servePreview(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 4<<20))
	if err != nil {
		writeError(writer, http.StatusBadRequest, &errorBody{Code: "read_body", Message: err.Error()})
		return
	}
	var payload commandRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, &errorBody{Code: "invalid_json", Message: err.Error()})
		return
	}
	handler.mu.Lock()
	rootPath := handler.rootPath
	handler.mu.Unlock()
	status, result, bodyErr := handler.executeCommand(rootPath, payload)
	if bodyErr != nil {
		writeError(writer, status, bodyErr)
		return
	}
	writeJSON(writer, status, responseEnvelope{OK: true, Data: result})
}

var _ = base64.StdEncoding
var _ = content.SourceBytes
