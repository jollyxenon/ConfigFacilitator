package mutate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// DeleteRequest describes one confirmed resource deletion and its independent destructive controls.
type DeleteRequest struct {
	Kind             ResourceKind
	ProjectReference string
	ColumnReference  string
	Reference        string
	Yes              bool
	Cascade          bool
	ForceTargets     bool
}

// ModeSelectionDependency identifies one persisted Mode selection that depends on a resource.
type ModeSelectionDependency struct {
	Mode     string `json:"mode"`
	Column   string `json:"column"`
	Strategy string `json:"strategy"`
}

// HistoryDependency identifies fields in one history record that depend on a resource.
type HistoryDependency struct {
	Index  int      `json:"index"`
	Fields []string `json:"fields"`
}

// ContextDependency identifies one PPID context that selects a Project.
type ContextDependency struct {
	PPID    int    `json:"ppid"`
	Project string `json:"project"`
}

// DependencyReport is the stable human/JSON inventory built before resource deletion.
type DependencyReport struct {
	Kind             ResourceKind                 `json:"kind"`
	Project          string                       `json:"project,omitempty"`
	Column           string                       `json:"column,omitempty"`
	Name             string                       `json:"name"`
	ModeSelections   []ModeSelectionDependency    `json:"modeSelections"`
	CurrentMappings  []repository.Mapping         `json:"currentMappings"`
	CurrentIntent    *repository.ApplyIntent      `json:"currentIntent,omitempty"`
	HistoryReferences []HistoryDependency         `json:"historyReferences"`
	PPIDContexts     []ContextDependency           `json:"ppidContexts"`
}

// Empty reports whether no persisted dependency category refers to the resource.
func (report DependencyReport) Empty() bool {
	return len(report.ModeSelections) == 0 && len(report.CurrentMappings) == 0 && report.CurrentIntent == nil && len(report.HistoryReferences) == 0 && len(report.PPIDContexts) == 0
}

// HumanMessage renders every non-empty dependency category without hiding JSON details.
func (report DependencyReport) HumanMessage() string {
	parts := []string{}
	if len(report.ModeSelections) > 0 {
		parts = append(parts, fmt.Sprintf("mode selections=%d", len(report.ModeSelections)))
	}
	if len(report.CurrentMappings) > 0 {
		parts = append(parts, fmt.Sprintf("current mappings=%d", len(report.CurrentMappings)))
	}
	if report.CurrentIntent != nil {
		parts = append(parts, "current intent=1")
	}
	if len(report.HistoryReferences) > 0 {
		parts = append(parts, fmt.Sprintf("history references=%d", len(report.HistoryReferences)))
	}
	if len(report.PPIDContexts) > 0 {
		parts = append(parts, fmt.Sprintf("PPID contexts=%d", len(report.PPIDContexts)))
	}
	if len(parts) == 0 {
		return "no dependencies"
	}
	return strings.Join(parts, ", ")
}

type deleteData struct {
	repo         repository.Repository
	request      DeleteRequest
	project      warehouse.Project
	column       warehouse.Column
	name         string
	projectIndex index.ProjectIndex
	columnIndex  index.ColumnIndex
	settingIndex index.SettingIndex
	modeIndex    index.ModeIndex
	current      repository.CurrentState
	history      []repository.HistoryEntry
	sessions     []renameSession
	paths        []string
	removePath   string
	removeLinks  []repository.Mapping
	saveProject  bool
	saveColumn   bool
	saveSetting  bool
	saveMode     bool
	saveCurrent  bool
	saveHistory  bool
}

// DeleteProject deletes one Project after confirmation and optional dependency cleanup.
func DeleteProject(repo repository.Repository, reference string, yes, cascade, forceTargets bool) (DependencyReport, error) {
	return Delete(repo, DeleteRequest{Kind: ProjectKind, Reference: reference, Yes: yes, Cascade: cascade, ForceTargets: forceTargets})
}

// DeleteColumn deletes one Column after confirmation and optional dependency cleanup.
func DeleteColumn(repo repository.Repository, projectReference, reference string, yes, cascade, forceTargets bool) (DependencyReport, error) {
	return Delete(repo, DeleteRequest{Kind: ColumnKind, ProjectReference: projectReference, Reference: reference, Yes: yes, Cascade: cascade, ForceTargets: forceTargets})
}

// DeleteSetting deletes one Setting after confirmation and optional dependency cleanup.
func DeleteSetting(repo repository.Repository, projectReference, columnReference, reference string, yes, cascade, forceTargets bool) (DependencyReport, error) {
	return Delete(repo, DeleteRequest{Kind: SettingKind, ProjectReference: projectReference, ColumnReference: columnReference, Reference: reference, Yes: yes, Cascade: cascade, ForceTargets: forceTargets})
}

// DeleteMode deletes one Mode after confirmation and optional intent cleanup.
func DeleteMode(repo repository.Repository, projectReference, reference string, yes, cascade, forceTargets bool) (DependencyReport, error) {
	return Delete(repo, DeleteRequest{Kind: ModeKind, ProjectReference: projectReference, Reference: reference, Yes: yes, Cascade: cascade, ForceTargets: forceTargets})
}

// Delete validates confirmation, dependencies, and ownership before committing any change.
func Delete(repo repository.Repository, request DeleteRequest) (DependencyReport, error) {
	if !request.Yes {
		return DependencyReport{}, refusal("confirmation_required", fmt.Sprintf("%s deletion requires --yes", request.Kind), nil)
	}
	report, data, err := buildDeletePlan(repo, request)
	if err != nil {
		return DependencyReport{}, err
	}
	if !report.Empty() && !request.Cascade {
		return report, &Error{Kind: RefusalError, Code: "dependencies_exist", Message: fmt.Sprintf("cannot delete %s %q without --cascade: %s", request.Kind, report.Name, report.HumanMessage()), Details: report}
	}
	if err := linker.New().ValidateRemovalMappings(data.removeLinks, request.ForceTargets); err != nil {
		return report, refusal("unsafe_target", err.Error(), err)
	}
	if err := commitDelete(data); err != nil {
		return report, err
	}
	return report, nil
}

// BuildDependencyReport resolves a resource and returns its complete deletion dependency inventory.
func BuildDependencyReport(repo repository.Repository, request DeleteRequest) (DependencyReport, error) {
	report, _, err := buildDeletePlan(repo, request)
	return report, err
}

// buildDeletePlan loads and rewrites every affected artifact in memory before ownership validation.
func buildDeletePlan(repo repository.Repository, request DeleteRequest) (DependencyReport, *deleteData, error) {
	if request.Kind != ProjectKind && request.Kind != ColumnKind && request.Kind != SettingKind && request.Kind != ModeKind {
		return DependencyReport{}, nil, invalid("invalid_delete_kind", fmt.Sprintf("unsupported delete kind %q", request.Kind), nil)
	}
	loaded, err := loadWarehouse(repo.RootPath)
	if err != nil {
		return DependencyReport{}, nil, err
	}
	data := &deleteData{repo: repo, request: request, projectIndex: loaded.ProjectIndex}
	if request.Kind == ProjectKind {
		data.project, err = loaded.ResolveProject(request.Reference)
	} else {
		data.project, err = loaded.ResolveProject(request.ProjectReference)
	}
	if err != nil {
		return DependencyReport{}, nil, missing("project_not_found", err.Error(), err)
	}
	data.name = data.project.Name
	if request.Kind == ColumnKind || request.Kind == SettingKind {
		columnReference := request.Reference
		if request.Kind == SettingKind {
			columnReference = request.ColumnReference
		}
		data.column, err = data.project.ResolveColumn(columnReference)
		if err != nil {
			return DependencyReport{}, nil, missing("column_not_found", err.Error(), err)
		}
		data.name = data.column.Name
	}
	if request.Kind == SettingKind {
		setting, resolveErr := data.column.ResolveSetting(request.Reference)
		if resolveErr != nil {
			return DependencyReport{}, nil, missing("setting_not_found", resolveErr.Error(), resolveErr)
		}
		data.name = setting.Name
	}
	if request.Kind == ModeKind {
		mode, resolveErr := data.project.ResolveMode(request.Reference)
		if resolveErr != nil {
			return DependencyReport{}, nil, missing("mode_not_found", resolveErr.Error(), resolveErr)
		}
		data.name = mode.Name
	}

	data.current, err = repo.LoadCurrentState(data.project.Name)
	if err != nil {
		return DependencyReport{}, nil, invalid("current_state", err.Error(), err)
	}
	data.history, err = repo.LoadHistory(data.project.Name)
	if err != nil {
		return DependencyReport{}, nil, invalid("history", err.Error(), err)
	}
	data.sessions, err = loadRenameSessions(repo)
	if err != nil {
		return DependencyReport{}, nil, invalid("session_context", err.Error(), err)
	}
	report := dependencyReport(data)
	prepareDeleteRewrite(data)
	enumerateDeletePaths(data)
	return report, data, nil
}

// dependencyReport scans Mode selections, current state, history, and PPID contexts deterministically.
func dependencyReport(data *deleteData) DependencyReport {
	report := DependencyReport{Kind: data.request.Kind, Project: data.project.Name, Name: data.name, ModeSelections: []ModeSelectionDependency{}, CurrentMappings: []repository.Mapping{}, HistoryReferences: []HistoryDependency{}, PPIDContexts: []ContextDependency{}}
	if data.request.Kind == SettingKind {
		report.Column = data.column.Name
	}
	for modeName, entry := range data.project.ModeIndex.Modes {
		for columnReference, selection := range entry.Columns {
			if selectionDepends(data, columnReference, selection) {
				report.ModeSelections = append(report.ModeSelections, ModeSelectionDependency{Mode: modeName, Column: columnReference, Strategy: selection.Strategy})
			}
		}
	}
	sort.Slice(report.ModeSelections, func(i, j int) bool {
		if report.ModeSelections[i].Mode == report.ModeSelections[j].Mode {
			return report.ModeSelections[i].Column < report.ModeSelections[j].Column
		}
		return report.ModeSelections[i].Mode < report.ModeSelections[j].Mode
	})
	for _, mapping := range data.current.Mappings {
		if mappingDepends(data, mapping) {
			report.CurrentMappings = append(report.CurrentMappings, mapping)
		}
	}
	if intentDepends(data, data.current.Intent) {
		report.CurrentIntent = cloneIntent(data.current.Intent)
	}
	for position, entry := range data.history {
		fields := []string{}
		if mappingsDepend(data, entry.PreviousMappings) { fields = append(fields, "previousMappings") }
		if mappingsDepend(data, entry.NextMappings) { fields = append(fields, "nextMappings") }
		if intentDepends(data, entry.PreviousIntent) { fields = append(fields, "previousIntent") }
		if intentDepends(data, entry.NextIntent) { fields = append(fields, "nextIntent") }
		if len(fields) > 0 {
			report.HistoryReferences = append(report.HistoryReferences, HistoryDependency{Index: position, Fields: fields})
		}
	}
	if data.request.Kind == ProjectKind {
		for _, session := range data.sessions {
			if session.record.Project == data.project.Name {
				report.PPIDContexts = append(report.PPIDContexts, ContextDependency{PPID: session.ppid, Project: session.record.Project})
			}
		}
	}
	return report
}

// selectionDepends reports whether one persisted Mode selection names the deleted Column or Setting.
func selectionDepends(data *deleteData, columnReference string, selection index.ModeColumnSelection) bool {
	if data.request.Kind != ColumnKind && data.request.Kind != SettingKind {
		return false
	}
	column, err := data.project.ResolveColumn(columnReference)
	if err != nil || column.Name != data.column.Name {
		return false
	}
	if data.request.Kind == ColumnKind {
		return true
	}
	for _, reference := range selection.Settings {
		setting, resolveErr := data.column.ResolveSetting(reference)
		if resolveErr == nil && setting.Name == data.name {
			return true
		}
	}
	return false
}

// mappingDepends reports whether one mapping source is equal to or below the deleted source root.
func mappingDepends(data *deleteData, mapping repository.Mapping) bool {
	var root string
	switch data.request.Kind {
	case ProjectKind:
		root = data.project.Path
	case ColumnKind:
		root = data.column.Path
	case SettingKind:
		root = filepath.Join(data.column.Path, data.name)
	default:
		return false
	}
	_, ok := rewriteDescendantPath(mapping.Source, root, root)
	return ok
}

// mappingsDepend reports whether any mapping in a state snapshot depends on the resource.
func mappingsDepend(data *deleteData, mappings []repository.Mapping) bool {
	for _, mapping := range mappings {
		if mappingDepends(data, mapping) { return true }
	}
	return false
}

// intentDepends reports semantic direct-Column or Mode intent dependence for the requested resource.
func intentDepends(data *deleteData, intent *repository.ApplyIntent) bool {
	if intent == nil { return false }
	switch data.request.Kind {
	case ProjectKind:
		return true
	case ModeKind:
		mode, err := data.project.ResolveMode(intent.Mode)
		return intent.Kind == "mode" && err == nil && mode.Name == data.name
	case ColumnKind:
		if intent.Kind == "column" {
			column, err := data.project.ResolveColumn(intent.Column)
			return err == nil && column.Name == data.column.Name
		}
		return modeIntentUsesSelection(data, intent)
	case SettingKind:
		if intent.Kind == "column" {
			column, err := data.project.ResolveColumn(intent.Column)
			if err != nil || column.Name != data.column.Name { return false }
			for _, reference := range intent.Settings {
				setting, resolveErr := data.column.ResolveSetting(reference)
				if resolveErr == nil && setting.Name == data.name { return true }
			}
			return false
		}
		return modeIntentUsesSelection(data, intent)
	default:
		return false
	}
}

// modeIntentUsesSelection reports whether a Mode intent selects the deleted Column or Setting.
func modeIntentUsesSelection(data *deleteData, intent *repository.ApplyIntent) bool {
	if intent == nil || intent.Kind != "mode" { return false }
	mode, err := data.project.ResolveMode(intent.Mode)
	if err != nil { return false }
	entry := data.project.ModeIndex.Modes[mode.Name]
	for columnReference, selection := range entry.Columns {
		if selectionDepends(data, columnReference, selection) { return true }
	}
	return false
}

// prepareDeleteRewrite removes only the requested resource and repairs dependent records for cascade commits.
func prepareDeleteRewrite(data *deleteData) {
	data.modeIndex = data.project.ModeIndex
	data.current = cloneCurrentState(data.current)
	data.history = cloneHistory(data.history)
	switch data.request.Kind {
	case ProjectKind:
		delete(data.projectIndex.Projects, data.project.Name)
		data.saveProject = true
		data.removePath = data.project.Path
		data.removeLinks = append([]repository.Mapping{}, data.current.Mappings...)
	case ColumnKind:
		data.columnIndex = data.project.ColumnIndex
		delete(data.columnIndex.Columns, data.column.Name)
		data.saveColumn = true
		data.removePath = data.column.Path
		removeModeSelections(data, false)
		rewriteDeleteState(data)
	case SettingKind:
		data.settingIndex = data.column.SettingIndex
		delete(data.settingIndex.Settings, data.name)
		data.saveSetting = true
		data.removePath = filepath.Join(data.column.Path, data.name)
		removeModeSelections(data, true)
		rewriteDeleteState(data)
	case ModeKind:
		delete(data.modeIndex.Modes, data.name)
		data.saveMode = true
		rewriteDeleteState(data)
	}
}

// removeModeSelections removes one Column or one Setting and drops empty explicit selections.
func removeModeSelections(data *deleteData, settingOnly bool) {
	changed := false
	for modeName, entry := range data.modeIndex.Modes {
		updated := cloneModeEntry(entry)
		for columnReference, selection := range entry.Columns {
			column, err := data.project.ResolveColumn(columnReference)
			if err != nil || column.Name != data.column.Name { continue }
			if !settingOnly {
				delete(updated.Columns, columnReference)
				changed = true
				continue
			}
			settings := []string{}
			for _, reference := range selection.Settings {
				setting, resolveErr := data.column.ResolveSetting(reference)
				if resolveErr == nil && setting.Name == data.name { changed = true; continue }
				settings = append(settings, reference)
			}
			if (selection.Strategy == modeStrategyCover || selection.Strategy == modeStrategyIncrement) && len(settings) == 0 {
				delete(updated.Columns, columnReference)
			} else {
				selection.Settings = settings
				updated.Columns[columnReference] = selection
			}
		}
		data.modeIndex.Modes[modeName] = updated
	}
	data.saveMode = changed
}

// rewriteDeleteState removes affected mappings and repairs current and historical intents.
func rewriteDeleteState(data *deleteData) {
	beforeMappings := append([]repository.Mapping{}, data.current.Mappings...)
	data.current.Mappings = filterDeleteMappings(data, data.current.Mappings)
	for _, mapping := range beforeMappings {
		if mappingDepends(data, mapping) { data.removeLinks = append(data.removeLinks, mapping) }
	}
	data.current.Intent = repairDeleteIntent(data, data.current.Intent)
	data.saveCurrent = currentStateChanged(data.current, data.project, data.repo)
	for index := range data.history {
		data.history[index].PreviousMappings = filterDeleteMappings(data, data.history[index].PreviousMappings)
		data.history[index].NextMappings = filterDeleteMappings(data, data.history[index].NextMappings)
		data.history[index].PreviousIntent = repairDeleteIntent(data, data.history[index].PreviousIntent)
		data.history[index].NextIntent = repairDeleteIntent(data, data.history[index].NextIntent)
	}
	data.saveHistory = historyChanged(data.history, data.project, data.repo)
}

// filterDeleteMappings retains unrelated mappings in their original order.
func filterDeleteMappings(data *deleteData, mappings []repository.Mapping) []repository.Mapping {
	result := []repository.Mapping{}
	for _, mapping := range mappings {
		if !mappingDepends(data, mapping) { result = append(result, mapping) }
	}
	return result
}

// repairDeleteIntent clears deleted Mode/Column intents and removes a deleted Setting from direct intent.
func repairDeleteIntent(data *deleteData, intent *repository.ApplyIntent) *repository.ApplyIntent {
	if intent == nil { return nil }
	updated := cloneIntent(intent)
	switch data.request.Kind {
	case ModeKind:
		if intentDepends(data, intent) { return nil }
	case ColumnKind:
		if updated.Kind == "column" {
			column, err := data.project.ResolveColumn(updated.Column)
			if err == nil && column.Name == data.column.Name { return nil }
		}
	case SettingKind:
		if updated.Kind == "column" {
			column, err := data.project.ResolveColumn(updated.Column)
			if err == nil && column.Name == data.column.Name {
				settings := []string{}
				for _, reference := range updated.Settings {
					setting, resolveErr := data.column.ResolveSetting(reference)
					if resolveErr == nil && setting.Name == data.name { continue }
					settings = append(settings, reference)
				}
				updated.Settings = settings
				if len(settings) == 0 { return nil }
			}
		}
	}
	return updated
}

// cloneCurrentState copies mutable state slices and extension fields for safe planning.
func cloneCurrentState(state repository.CurrentState) repository.CurrentState {
	return repository.CurrentState{Mappings: append([]repository.Mapping{}, state.Mappings...), Intent: cloneIntent(state.Intent), Extra: cloneRawMap(state.Extra)}
}

// cloneHistory copies every mutable history field before cascade rewriting.
func cloneHistory(entries []repository.HistoryEntry) []repository.HistoryEntry {
	result := make([]repository.HistoryEntry, len(entries))
	for index, entry := range entries {
		entry.PreviousMappings = append([]repository.Mapping{}, entry.PreviousMappings...)
		entry.NextMappings = append([]repository.Mapping{}, entry.NextMappings...)
		entry.PreviousIntent = cloneIntent(entry.PreviousIntent)
		entry.NextIntent = cloneIntent(entry.NextIntent)
		entry.Extra = cloneRawMap(entry.Extra)
		result[index] = entry
	}
	return result
}

// enumerateDeletePaths creates a complete source, index, runtime, context, and target rollback inventory.
func enumerateDeletePaths(data *deleteData) {
	paths := []string{data.removePath}
	add := func(path string) { if path != "" && !containsPath(paths, path) { paths = append(paths, path) } }
	switch data.request.Kind {
	case ProjectKind:
		add(data.repo.ProjectIndexPath())
		for _, session := range data.sessions {
			if session.record.Project == data.project.Name { add(data.repo.SessionPath(session.ppid)) }
		}
	case ColumnKind:
		add(data.repo.ColumnIndexPath(data.project.Name))
		add(data.repo.ModeIndexPath(data.project.Name))
		add(data.repo.CurrentStatePath(data.project.Name))
		add(data.repo.HistoryPath(data.project.Name))
	case SettingKind:
		add(data.repo.SettingIndexPath(data.project.Name, data.column.Name))
		add(data.repo.ModeIndexPath(data.project.Name))
		add(data.repo.CurrentStatePath(data.project.Name))
		add(data.repo.HistoryPath(data.project.Name))
	case ModeKind:
		add(data.repo.ModeIndexPath(data.project.Name))
		add(data.repo.CurrentStatePath(data.project.Name))
		add(data.repo.HistoryPath(data.project.Name))
	}
	for _, mapping := range data.removeLinks { add(mapping.Target) }
	sort.Strings(paths)
	data.paths = compactSnapshotPaths(paths)
}

// commitDelete executes repository rewrites, target reclamation, context cleanup, and source removal in one transaction.
func commitDelete(data *deleteData) error {
	err := data.repo.WithMutation("delete-"+string(data.request.Kind), data.paths, func() error {
		if len(data.removeLinks) > 0 {
			if err := linker.New().ApplyRemovalMappings(data.removeLinks, data.request.ForceTargets); err != nil { return err }
		}
		if data.saveProject {
			if err := data.repo.SaveProjectIndex(data.projectIndex); err != nil { return err }
		}
		if data.saveColumn {
			if err := data.repo.SaveColumnIndex(data.project.Name, data.columnIndex); err != nil { return err }
		}
		if data.saveSetting {
			if err := data.repo.SaveSettingIndex(data.project.Name, data.column.Name, data.settingIndex); err != nil { return err }
		}
		if data.saveMode {
			if err := data.repo.SaveModeIndex(data.project.Name, data.modeIndex); err != nil { return err }
		}
		if data.saveCurrent {
			if err := data.repo.SaveCurrentState(data.project.Name, data.current); err != nil { return err }
		}
		if data.saveHistory {
			if err := data.repo.SaveHistory(data.project.Name, data.history); err != nil { return err }
		}
		if data.request.Kind == ProjectKind {
			for _, session := range data.sessions {
				if session.record.Project == data.project.Name {
					if err := os.Remove(data.repo.SessionPath(session.ppid)); err != nil && !os.IsNotExist(err) { return err }
				}
			}
		}
		if data.removePath != "" {
			if err := removeDeletePath(data.removePath); err != nil { return err }
		}
		return nil
	})
	if err != nil {
		return persistence("delete_"+string(data.request.Kind), fmt.Sprintf("delete %s %q", data.request.Kind, data.name), err)
	}
	return nil
}

// removeDeletePath removes one resource source without following symlinks.
func removeDeletePath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) { return nil }
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 { return os.RemoveAll(path) }
	return os.Remove(path)
}
