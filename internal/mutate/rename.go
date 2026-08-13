package mutate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// RenameRequest describes one canonical resource rename and its ownership policy.
type RenameRequest struct {
	Kind             ResourceKind
	ProjectReference string
	ColumnReference  string
	OldReference     string
	NewName          string
	ForceTargets     bool
	PlanOptions      planner.PlanOptions
}

// RenameMove records the source move that a rename plan will attempt.
type RenameMove struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Exists bool   `json:"exists"`
}

// RenameReference records one schema-defined reference rewritten by a plan.
type RenameReference struct {
	Owner string `json:"owner"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// RenameContext records one PPID context rewritten by a Project rename.
type RenameContext struct {
	PPID int    `json:"ppid"`
	From string `json:"from"`
	To   string `json:"to"`
}

// RenamePlan is the complete pre-commit inventory for one resource rename.
type RenamePlan struct {
	Kind             ResourceKind            `json:"kind"`
	Project          string                  `json:"project,omitempty"`
	Column           string                  `json:"column,omitempty"`
	OldName          string                  `json:"oldName"`
	NewName          string                  `json:"newName"`
	Moves            []RenameMove            `json:"moves"`
	Paths            []string                `json:"paths"`
	IndexReferences  []RenameReference       `json:"indexReferences"`
	IntentReferences []RenameReference       `json:"intentReferences"`
	HistoryEntries   int                     `json:"historyEntries"`
	Contexts         []RenameContext         `json:"contexts"`
	ManagedLinks     []linker.MappingRewrite `json:"managedLinks"`
	data             *renameData
}

type renameData struct {
	repo             repository.Repository
	request          RenameRequest
	project          warehouse.Project
	projectIndex     index.ProjectIndex
	columnIndex      index.ColumnIndex
	settingIndex     index.SettingIndex
	modeIndex        index.ModeIndex
	current          repository.CurrentState
	history          []repository.HistoryEntry
	sessions         []renameSession
	sourceMove       RenameMove
	saveProjectIndex bool
	saveColumnIndex  bool
	saveSettingIndex bool
	saveModeIndex    bool
	saveCurrent      bool
	saveHistory      bool
	oldSource        string
	newSource        string
}

type renameSession struct {
	ppid   int
	record repository.SessionRecord
}

// RenameProject builds and commits a Project rename plan.
func RenameProject(repo repository.Repository, oldReference, newName string, forceTargets bool, options planner.PlanOptions) error {
	return Rename(repo, RenameRequest{Kind: ProjectKind, OldReference: oldReference, NewName: newName, ForceTargets: forceTargets, PlanOptions: options})
}

// RenameColumn builds and commits a Column rename plan.
func RenameColumn(repo repository.Repository, projectReference, oldReference, newName string, forceTargets bool, options planner.PlanOptions) error {
	return Rename(repo, RenameRequest{Kind: ColumnKind, ProjectReference: projectReference, OldReference: oldReference, NewName: newName, ForceTargets: forceTargets, PlanOptions: options})
}

// RenameSetting builds and commits a Setting rename plan.
func RenameSetting(repo repository.Repository, projectReference, columnReference, oldReference, newName string, forceTargets bool, options planner.PlanOptions) error {
	return Rename(repo, RenameRequest{Kind: SettingKind, ProjectReference: projectReference, ColumnReference: columnReference, OldReference: oldReference, NewName: newName, ForceTargets: forceTargets, PlanOptions: options})
}

// RenameMode builds and commits a Mode rename plan.
func RenameMode(repo repository.Repository, projectReference, oldReference, newName string, forceTargets bool, options planner.PlanOptions) error {
	return Rename(repo, RenameRequest{Kind: ModeKind, ProjectReference: projectReference, OldReference: oldReference, NewName: newName, ForceTargets: forceTargets, PlanOptions: options})
}

// Rename builds a complete plan before changing any repository or managed target.
func Rename(repo repository.Repository, request RenameRequest) error {
	plan, err := BuildRenamePlan(repo, request)
	if err != nil {
		return err
	}
	return commitRenamePlan(plan)
}

// BuildRenamePlan resolves canonical identities and enumerates every affected artifact.
func BuildRenamePlan(repo repository.Repository, request RenameRequest) (RenamePlan, error) {
	if request.Kind != ProjectKind && request.Kind != ColumnKind && request.Kind != SettingKind && request.Kind != ModeKind {
		return RenamePlan{}, invalid("invalid_rename_kind", fmt.Sprintf("unsupported rename kind %q", request.Kind), nil)
	}
	if err := ValidateCanonicalName(request.Kind, request.NewName); err != nil {
		return RenamePlan{}, err
	}
	if request.OldReference == "" {
		return RenamePlan{}, invalid("rename_source_required", "rename source cannot be empty", nil)
	}
	loaded, err := loadWarehouse(repo.RootPath)
	if err != nil {
		return RenamePlan{}, err
	}
	plan := RenamePlan{Kind: request.Kind, NewName: request.NewName, Moves: []RenameMove{}, Paths: []string{}, IndexReferences: []RenameReference{}, IntentReferences: []RenameReference{}, Contexts: []RenameContext{}, ManagedLinks: []linker.MappingRewrite{}}
	data := &renameData{repo: repo, request: request, projectIndex: loaded.ProjectIndex}
	plan.data = data

	var project warehouse.Project
	var column warehouse.Column
	var oldName string
	switch request.Kind {
	case ProjectKind:
		project, err = loaded.ResolveProject(request.OldReference)
		if err != nil {
			return RenamePlan{}, missing("project_not_found", err.Error(), err)
		}
		oldName = project.Name
	case ColumnKind, SettingKind, ModeKind:
		project, err = loaded.ResolveProject(request.ProjectReference)
		if err != nil {
			return RenamePlan{}, missing("project_not_found", err.Error(), err)
		}
		if request.Kind == ColumnKind {
			column, err = project.ResolveColumn(request.OldReference)
		} else if request.Kind == SettingKind {
			column, err = project.ResolveColumn(request.ColumnReference)
		} else {
			_, err = project.ResolveMode(request.OldReference)
		}
		if err != nil {
			return RenamePlan{}, missing(renameNotFoundCode(request.Kind), err.Error(), err)
		}
		if request.Kind == ColumnKind {
			column, _ = project.ResolveColumn(request.OldReference)
			oldName = column.Name
		} else if request.Kind == SettingKind {
			setting, resolveErr := column.ResolveSetting(request.OldReference)
			if resolveErr != nil {
				return RenamePlan{}, missing("setting_not_found", resolveErr.Error(), resolveErr)
			}
			oldName = setting.Name
		} else {
			mode, resolveErr := project.ResolveMode(request.OldReference)
			if resolveErr != nil {
				return RenamePlan{}, missing("mode_not_found", resolveErr.Error(), resolveErr)
			}
			oldName = mode.Name
		}
	}
	if oldName == request.NewName {
		return RenamePlan{}, conflict("rename_same_name", fmt.Sprintf("%s %q already has that canonical name", request.Kind, oldName), nil)
	}
	plan.Project = project.Name
	plan.Column = column.Name
	plan.OldName = oldName
	data.project = project
	data.oldSource, data.newSource = renameSourcePaths(project, column, request.Kind, oldName, request.NewName)
	data.sourceMove = RenameMove{From: data.oldSource, To: data.newSource, Exists: pathExists(data.oldSource)}
	if data.oldSource != "" {
		plan.Moves = append(plan.Moves, data.sourceMove)
	}
	if data.sourceMove.Exists && pathExists(data.newSource) {
		return RenamePlan{}, conflict("rename_destination_exists", fmt.Sprintf("rename destination %q already exists", data.newSource), nil)
	}
	if err := validateRenameIdentity(loaded, project, column, request.Kind, oldName, request.NewName); err != nil {
		return RenamePlan{}, err
	}
	plan.IndexReferences = append(plan.IndexReferences, RenameReference{Owner: renameIndexOwner(request.Kind, project.Name, column.Name), From: oldName, To: request.NewName})
	if err := prepareRenameData(&plan, loaded, project, column, oldName); err != nil {
		return RenamePlan{}, err
	}
	if err := enumerateRenamePaths(&plan); err != nil {
		return RenamePlan{}, err
	}
	if err := validateRenameLinks(&plan); err != nil {
		return RenamePlan{}, err
	}
	return plan, nil
}

// renameIndexOwner identifies the owning canonical-key map for a rename.
func renameIndexOwner(kind ResourceKind, projectName, columnName string) string {
	switch kind {
	case ProjectKind:
		return "ProjectIndex"
	case ColumnKind:
		return "project." + projectName + ".ColumnIndex"
	case SettingKind:
		return "project." + projectName + ".column." + columnName + ".SettingIndex.settings"
	case ModeKind:
		return "project." + projectName + ".ModeIndex"
	default:
		return "index"
	}
}

// renameNotFoundCode returns the stable missing-resource code for one kind.
func renameNotFoundCode(kind ResourceKind) string {
	switch kind {
	case ColumnKind:
		return "column_not_found"
	case SettingKind:
		return "column_not_found"
	case ModeKind:
		return "mode_not_found"
	default:
		return "resource_not_found"
	}
}

// validateRenameIdentity checks destination names, aliases, paths, and per-scope collisions.
func validateRenameIdentity(loaded warehouse.Warehouse, project warehouse.Project, column warehouse.Column, kind ResourceKind, oldName, newName string) error {
	if pathExists(renamePathForKind(project, column, kind, newName)) {
		return conflict("rename_destination_exists", fmt.Sprintf("%s destination %q already exists", kind, newName), nil)
	}
	switch kind {
	case ProjectKind:
		return ValidateIdentityScope(kind, Identity{CanonicalName: newName, Aliases: project.Metadata.Aliases}, projectIdentities(loaded), oldName)
	case ColumnKind:
		return ValidateIdentityScope(kind, Identity{CanonicalName: newName, Aliases: column.Metadata.Aliases}, columnIdentities(project), oldName)
	case SettingKind:
		return ValidateIdentityScope(kind, Identity{CanonicalName: newName, Aliases: column.Settings[oldName].Metadata.Aliases}, settingIdentities(column), oldName)
	case ModeKind:
		return ValidateIdentityScope(kind, Identity{CanonicalName: newName, Aliases: project.Modes[oldName].Metadata.Aliases}, modeIdentities(project), oldName)
	default:
		return nil
	}
}

// renameSourcePaths derives canonical source paths without creating missing resources.
func renameSourcePaths(project warehouse.Project, column warehouse.Column, kind ResourceKind, oldName, newName string) (string, string) {
	switch kind {
	case ProjectKind:
		return filepath.Join(filepath.Dir(project.Path), oldName), filepath.Join(filepath.Dir(project.Path), newName)
	case ColumnKind:
		return filepath.Join(filepath.Dir(column.Path), oldName), filepath.Join(filepath.Dir(column.Path), newName)
	case SettingKind:
		return filepath.Join(column.Path, oldName), filepath.Join(column.Path, newName)
	default:
		return "", ""
	}
}

// renamePathForKind returns the filesystem destination for collision checking.
func renamePathForKind(project warehouse.Project, column warehouse.Column, kind ResourceKind, newName string) string {
	from, to := renameSourcePaths(project, column, kind, "", newName)
	_ = from
	return to
}

// prepareRenameData rewrites all schema-defined records in memory before commit.
func prepareRenameData(plan *RenamePlan, loaded warehouse.Warehouse, project warehouse.Project, column warehouse.Column, oldName string) error {
	data := plan.data
	request := data.request
	current, err := data.repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error(), err)
	}
	history, err := data.repo.LoadHistory(project.Name)
	if err != nil {
		return invalid("history", err.Error(), err)
	}
	data.current = current
	data.history = history
	data.saveCurrent = pathExists(data.repo.CurrentStatePath(project.Name))
	data.saveHistory = pathExists(data.repo.HistoryPath(project.Name))

	switch request.Kind {
	case ProjectKind:
		entry := data.projectIndex.Projects[oldName]
		entry.WarehouseName = request.NewName
		data.projectIndex.Projects[request.NewName] = entry
		delete(data.projectIndex.Projects, oldName)
		data.saveProjectIndex = true
		sessions, err := loadRenameSessions(data.repo)
		if err != nil {
			return invalid("session_context", err.Error(), err)
		}
		data.sessions = sessions
		for index := range data.sessions {
			if data.sessions[index].record.Project == oldName {
				plan.Contexts = append(plan.Contexts, RenameContext{PPID: data.sessions[index].ppid, From: oldName, To: request.NewName})
				data.sessions[index].record.Project = request.NewName
			}
		}
	case ColumnKind:
		data.columnIndex = project.ColumnIndex
		entry := data.columnIndex.Columns[oldName]
		entry.WarehouseName = request.NewName
		data.columnIndex.Columns[request.NewName] = entry
		delete(data.columnIndex.Columns, oldName)
		data.saveColumnIndex = !project.Missing
		data.modeIndex = project.ModeIndex
		data.saveModeIndex = pathExists(data.repo.ModeIndexPath(project.Name))
		if err := rewriteModeColumns(&data.modeIndex, project, oldName, request.NewName, &plan.IndexReferences); err != nil {
			return err
		}
		if data.current.Intent != nil && intentResolvesColumn(project, data.current.Intent, oldName) {
			from := data.current.Intent.Column
			data.current.Intent = cloneIntent(data.current.Intent)
			data.current.Intent.Column = request.NewName
			plan.IntentReferences = append(plan.IntentReferences, RenameReference{Owner: "current.intent.column", From: from, To: request.NewName})
		}
		for index := range data.history {
			rewriteHistoryIntentColumn(&data.history[index], project, oldName, request.NewName, &plan.IntentReferences)
		}
	case SettingKind:
		data.settingIndex = column.SettingIndex
		entry := data.settingIndex.Settings[oldName]
		entry.WarehouseName = request.NewName
		data.settingIndex.Settings[request.NewName] = entry
		delete(data.settingIndex.Settings, oldName)
		data.saveSettingIndex = !column.Missing
		data.modeIndex = project.ModeIndex
		data.saveModeIndex = pathExists(data.repo.ModeIndexPath(project.Name))
		if err := rewriteModeSettings(&data.modeIndex, project, column.Name, oldName, request.NewName, &plan.IndexReferences); err != nil {
			return err
		}
		if err := rewriteIntentSettings(&data.current, column, oldName, request.NewName, &plan.IntentReferences); err != nil {
			return err
		}
		for index := range data.history {
			if err := rewriteHistoryIntentSettings(&data.history[index], column, oldName, request.NewName, &plan.IntentReferences); err != nil {
				return err
			}
		}
	case ModeKind:
		data.modeIndex = project.ModeIndex
		entry := data.modeIndex.Modes[oldName]
		entry.WarehouseName = request.NewName
		data.modeIndex.Modes[request.NewName] = entry
		delete(data.modeIndex.Modes, oldName)
		data.saveModeIndex = !project.Missing
		if err := rewriteIntentMode(&data.current, project, oldName, request.NewName, &plan.IntentReferences); err != nil {
			return err
		}
		for index := range data.history {
			rewriteHistoryIntentMode(&data.history[index], project, oldName, request.NewName, &plan.IntentReferences)
		}
	}

	targetMap := map[string]string{}
	if request.Kind == SettingKind && (len(data.current.Mappings) > 0 || len(data.history) > 0) {
		var err error
		targetMap, err = settingRenameTargetMap(project, column, oldName, request.NewName, request.PlanOptions)
		if err != nil {
			return invalid("rename_target_plan", err.Error(), err)
		}
	}
	currentChanged := false
	data.current.Mappings, currentChanged = rewriteMappings(data.current.Mappings, data.oldSource, data.newSource, targetMap, &plan.ManagedLinks)
	for index := range data.history {
		data.history[index].PreviousMappings, _ = rewriteMappings(data.history[index].PreviousMappings, data.oldSource, data.newSource, targetMap, nil)
		data.history[index].NextMappings, _ = rewriteMappings(data.history[index].NextMappings, data.oldSource, data.newSource, targetMap, nil)
	}
	plan.HistoryEntries = len(data.history)
	data.saveCurrent = data.saveCurrent && (currentChanged || currentStateChanged(data.current, project, data.repo))
	data.saveHistory = data.saveHistory && historyChanged(data.history, project, data.repo)
	return nil
}

// enumerateRenamePaths lists index, runtime, session, source, target, and destination paths for rollback.
func enumerateRenamePaths(plan *RenamePlan) error {
	data := plan.data
	paths := []string{data.repo.ProjectIndexPath()}
	add := func(path string) {
		if path != "" && !containsPath(paths, path) {
			paths = append(paths, path)
		}
	}
	if data.sourceMove.From != "" {
		add(data.sourceMove.From)
		add(data.sourceMove.To)
	}
	if data.request.Kind == ColumnKind || data.request.Kind == SettingKind || data.request.Kind == ModeKind {
		add(data.repo.ColumnIndexPath(data.project.Name))
		add(data.repo.ModeIndexPath(data.project.Name))
	}
	if data.request.Kind == SettingKind {
		add(data.repo.SettingIndexPath(data.project.Name, plan.Column))
	}
	add(data.repo.CurrentStatePath(data.project.Name))
	add(data.repo.HistoryPath(data.project.Name))
	for _, context := range data.sessions {
		if context.record.Project == plan.NewName || context.record.Project == plan.OldName {
			add(data.repo.SessionPath(context.ppid))
		}
	}
	for _, rewrite := range plan.ManagedLinks {
		add(rewrite.Previous.Target)
		add(rewrite.Next.Target)
	}
	sort.Strings(paths)
	plan.Paths = compactSnapshotPaths(paths)
	return nil
}

// compactSnapshotPaths removes descendants already covered by an ancestor snapshot.
func compactSnapshotPaths(paths []string) []string {
	result := []string{}
	for _, candidate := range paths {
		covered := false
		for _, ancestor := range result {
			relative, err := filepath.Rel(ancestor, candidate)
			if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, candidate)
		}
	}
	return result
}

// validateRenameLinks rejects active missing sources and unsafe target ownership before commit.
func validateRenameLinks(plan *RenamePlan) error {
	data := plan.data
	for _, rewrite := range plan.ManagedLinks {
		if !pathExists(rewrite.Previous.Source) {
			return missing("active_source_missing", fmt.Sprintf("active mapping source %q is missing", rewrite.Previous.Source), nil)
		}
	}
	if err := linker.New().ValidateRenameMappings(plan.ManagedLinks, data.request.ForceTargets); err != nil {
		return refusal("unsafe_target", err.Error(), err)
	}
	return nil
}

// commitRenamePlan executes one prepared transaction and rolls every layer back on failure.
func commitRenamePlan(plan RenamePlan) error {
	data := plan.data
	if data == nil {
		return persistence("rename_plan", "rename plan has no executable state", fmt.Errorf("missing executable plan"))
	}
	projectName := data.project.Name
	if plan.Kind == ProjectKind {
		projectName = plan.NewName
	}
	err := data.repo.WithMutation("rename-"+string(plan.Kind), plan.Paths, func() error {
		if data.sourceMove.Exists {
			if err := os.Rename(data.sourceMove.From, data.sourceMove.To); err != nil {
				return err
			}
		}
		if data.saveProjectIndex {
			if err := data.repo.SaveProjectIndex(data.projectIndex); err != nil {
				return err
			}
		}
		if data.saveColumnIndex {
			if err := data.repo.SaveColumnIndex(projectName, data.columnIndex); err != nil {
				return err
			}
		}
		if data.saveSettingIndex {
			if err := data.repo.SaveSettingIndex(projectName, plan.Column, data.settingIndex); err != nil {
				return err
			}
		}
		if data.saveModeIndex {
			if err := data.repo.SaveModeIndex(projectName, data.modeIndex); err != nil {
				return err
			}
		}
		if data.saveCurrent {
			if err := data.repo.SaveCurrentState(projectName, data.current); err != nil {
				return err
			}
		}
		if data.saveHistory {
			if err := data.repo.SaveHistory(projectName, data.history); err != nil {
				return err
			}
		}
		for _, context := range data.sessions {
			if context.record.Project == plan.NewName && plan.Kind == ProjectKind {
				if err := data.repo.SaveSession(context.ppid, context.record); err != nil {
					return err
				}
			}
		}
		if len(plan.ManagedLinks) > 0 {
			if err := linker.New().ApplyRenameMappings(plan.ManagedLinks, data.request.ForceTargets); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return persistence("rename_"+string(plan.Kind), fmt.Sprintf("rename %s %q to %q", plan.Kind, plan.OldName, plan.NewName), err)
	}
	return nil
}

// rewriteModeColumns rewrites all persisted references resolving to one renamed Column.
func rewriteModeColumns(modeIndex *index.ModeIndex, project warehouse.Project, oldName, newName string, refs *[]RenameReference) error {
	for modeName, entry := range modeIndex.Modes {
		updated := cloneModeEntry(entry)
		for key, selection := range entry.Columns {
			column, err := project.ResolveColumn(key)
			if err != nil || column.Name != oldName {
				continue
			}
			if existing, ok := updated.Columns[newName]; ok && key != newName {
				_ = existing
				return conflict("mode_column_reference_conflict", fmt.Sprintf("mode %q contains multiple references to column %q", modeName, oldName), nil)
			}
			delete(updated.Columns, key)
			updated.Columns[newName] = selection
			*refs = append(*refs, RenameReference{Owner: "mode." + modeName + ".columns", From: key, To: newName})
		}
		modeIndex.Modes[modeName] = updated
	}
	return nil
}

// rewriteModeSettings rewrites Setting references inside selections for one Column.
func rewriteModeSettings(modeIndex *index.ModeIndex, project warehouse.Project, columnName, oldName, newName string, refs *[]RenameReference) error {
	column, err := project.ResolveColumn(columnName)
	if err != nil {
		return err
	}
	for modeName, entry := range modeIndex.Modes {
		updated := cloneModeEntry(entry)
		for columnReference, selection := range entry.Columns {
			resolvedColumn, resolveErr := project.ResolveColumn(columnReference)
			if resolveErr != nil || resolvedColumn.Name != column.Name {
				continue
			}
			selection.Settings = append([]string{}, selection.Settings...)
			for index, settingReference := range selection.Settings {
				setting, resolveErr := column.ResolveSetting(settingReference)
				if resolveErr != nil || setting.Name != oldName {
					continue
				}
				selection.Settings[index] = newName
				*refs = append(*refs, RenameReference{Owner: "mode." + modeName + ".columns." + columnReference + ".settings", From: settingReference, To: newName})
			}
			updated.Columns[columnReference] = selection
		}
		modeIndex.Modes[modeName] = updated
	}
	return nil
}

// rewriteIntentSettings rewrites Setting references in one current apply intent.
func rewriteIntentSettings(state *repository.CurrentState, column warehouse.Column, oldName, newName string, refs *[]RenameReference) error {
	if state.Intent == nil {
		return nil
	}
	intent := cloneIntent(state.Intent)
	columnMatches := intent.Column == column.Name
	for _, alias := range column.Metadata.Aliases {
		columnMatches = columnMatches || intent.Column == alias
	}
	if intent.Kind == "column" && columnMatches {
		for index, reference := range intent.Settings {
			setting, err := column.ResolveSetting(reference)
			if err == nil && setting.Name == oldName {
				intent.Settings[index] = newName
				*refs = append(*refs, RenameReference{Owner: "current.intent.settings", From: reference, To: newName})
			}
		}
	}
	state.Intent = intent
	return nil
}

// rewriteIntentMode rewrites a Mode reference in one current apply intent.
func rewriteIntentMode(state *repository.CurrentState, project warehouse.Project, oldName, newName string, refs *[]RenameReference) error {
	if state.Intent == nil || state.Intent.Mode == "" {
		return nil
	}
	mode, err := project.ResolveMode(state.Intent.Mode)
	if err == nil && mode.Name == oldName {
		state.Intent = cloneIntent(state.Intent)
		state.Intent.Mode = newName
		*refs = append(*refs, RenameReference{Owner: "current.intent.mode", From: mode.Name, To: newName})
	}
	return nil
}

// rewriteHistoryIntentColumn rewrites a Column reference in both history intent directions.
func rewriteHistoryIntentColumn(entry *repository.HistoryEntry, project warehouse.Project, oldName, newName string, refs *[]RenameReference) {
	for _, intent := range []*repository.ApplyIntent{entry.PreviousIntent, entry.NextIntent} {
		if intentResolvesColumn(project, intent, oldName) {
			from := intent.Column
			intentCopy := cloneIntent(intent)
			intentCopy.Column = newName
			if intent == entry.PreviousIntent {
				entry.PreviousIntent = intentCopy
			} else {
				entry.NextIntent = intentCopy
			}
			*refs = append(*refs, RenameReference{Owner: "history.intent.column", From: from, To: newName})
		}
	}
}

// intentResolvesColumn reports whether one direct intent names a canonical Column or alias.
func intentResolvesColumn(project warehouse.Project, intent *repository.ApplyIntent, canonicalName string) bool {
	if intent == nil || intent.Column == "" {
		return false
	}
	column, err := project.ResolveColumn(intent.Column)
	return err == nil && column.Name == canonicalName
}

// rewriteHistoryIntentSettings rewrites Setting references in both history intent directions.
func rewriteHistoryIntentSettings(entry *repository.HistoryEntry, column warehouse.Column, oldName, newName string, refs *[]RenameReference) error {
	for _, intent := range []*repository.ApplyIntent{entry.PreviousIntent, entry.NextIntent} {
		if intent == nil || intent.Kind != "column" {
			continue
		}
		columnMatches := intent.Column == column.Name
		for _, alias := range column.Metadata.Aliases {
			columnMatches = columnMatches || intent.Column == alias
		}
		if !columnMatches {
			continue
		}
		updated := cloneIntent(intent)
		for index, reference := range updated.Settings {
			setting, err := column.ResolveSetting(reference)
			if err == nil && setting.Name == oldName {
				updated.Settings[index] = newName
				*refs = append(*refs, RenameReference{Owner: "history.intent.settings", From: reference, To: newName})
			}
		}
		if intent == entry.PreviousIntent {
			entry.PreviousIntent = updated
		} else {
			entry.NextIntent = updated
		}
	}
	return nil
}

// rewriteHistoryIntentMode rewrites a Mode reference in both history intent directions.
func rewriteHistoryIntentMode(entry *repository.HistoryEntry, project warehouse.Project, oldName, newName string, refs *[]RenameReference) {
	for _, intent := range []*repository.ApplyIntent{entry.PreviousIntent, entry.NextIntent} {
		if intent == nil || intent.Mode == "" {
			continue
		}
		mode, err := project.ResolveMode(intent.Mode)
		if err != nil || mode.Name != oldName {
			continue
		}
		updated := cloneIntent(intent)
		updated.Mode = newName
		if intent == entry.PreviousIntent {
			entry.PreviousIntent = updated
		} else {
			entry.NextIntent = updated
		}
		*refs = append(*refs, RenameReference{Owner: "history.intent.mode", From: mode.Name, To: newName})
	}
}

// rewriteMappings rewrites source descendants and any Setting-derived target names.
func rewriteMappings(mappings []repository.Mapping, oldRoot, newRoot string, targetMap map[string]string, links *[]linker.MappingRewrite) ([]repository.Mapping, bool) {
	updated := make([]repository.Mapping, len(mappings))
	changed := false
	for index, mapping := range mappings {
		next := mapping
		if rewritten, ok := rewriteDescendantPath(mapping.Source, oldRoot, newRoot); ok {
			next.Source = rewritten
		}
		if target, ok := targetMap[mapping.Source+"\x00"+mapping.Target]; ok {
			next.Target = target
		}
		if next != mapping {
			changed = true
		}
		updated[index] = next
		if links != nil && next != mapping {
			*links = append(*links, linker.MappingRewrite{Previous: mapping, Next: next})
		}
	}
	return updated, changed
}

// rewriteDescendantPath rewrites only a path equal to or below one canonical source root.
func rewriteDescendantPath(value, oldRoot, newRoot string) (string, bool) {
	if oldRoot == "" || newRoot == "" {
		return value, false
	}
	relative, err := filepath.Rel(filepath.Clean(oldRoot), filepath.Clean(value))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return value, false
	}
	return filepath.Join(newRoot, relative), true
}

// settingRenameTargetMap computes fixed-versus-derived target changes from old and new models.
func settingRenameTargetMap(project warehouse.Project, column warehouse.Column, oldName, newName string, options planner.PlanOptions) (map[string]string, error) {
	if column.SettingIndex.TargetNumber == 0 {
		return map[string]string{}, nil
	}
	oldMappings, err := planner.PlanColumnMappings(project, column.Name, []string{oldName}, options)
	if err != nil {
		return nil, err
	}
	clonedProject := cloneProject(project)
	clonedColumn := clonedProject.Columns[column.Name]
	oldSetting := clonedColumn.Settings[oldName]
	newSetting := oldSetting
	newSetting.Name, newSetting.WarehouseName = newName, newName
	newSetting.Path = filepath.Join(clonedColumn.Path, newName)
	delete(clonedColumn.Settings, oldName)
	clonedColumn.Settings[newName] = newSetting
	entry := clonedColumn.SettingIndex.Settings[oldName]
	entry.WarehouseName = newName
	delete(clonedColumn.SettingIndex.Settings, oldName)
	clonedColumn.SettingIndex.Settings[newName] = entry
	clonedProject.Columns[column.Name] = clonedColumn
	newMappings, err := planner.PlanColumnMappings(clonedProject, column.Name, []string{newName}, options)
	if err != nil {
		return nil, err
	}
	if len(oldMappings) != len(newMappings) {
		return nil, fmt.Errorf("rename target plan changed target count")
	}
	result := map[string]string{}
	for index := range oldMappings {
		result[oldMappings[index].Source+"\x00"+oldMappings[index].Target] = newMappings[index].Target
	}
	return result, nil
}

// cloneProject copies the maps needed for a temporary Setting-derived target plan.
func cloneProject(project warehouse.Project) warehouse.Project {
	cloned := project
	cloned.Columns = map[string]warehouse.Column{}
	for name, column := range project.Columns {
		copied := column
		copied.Settings = map[string]warehouse.Setting{}
		for settingName, setting := range column.Settings {
			copied.Settings[settingName] = setting
		}
		copied.SettingIndex.Settings = map[string]index.SettingEntry{}
		for settingName, entry := range column.SettingIndex.Settings {
			copied.SettingIndex.Settings[settingName] = entry
		}
		cloned.Columns[name] = copied
	}
	return cloned
}

// cloneModeEntry copies mutable Mode maps and extension fields without dropping unknown data.
func cloneModeEntry(entry index.ModeEntry) index.ModeEntry {
	copied := entry
	copied.Columns = map[string]index.ModeColumnSelection{}
	for key, selection := range entry.Columns {
		selection.Settings = append([]string{}, selection.Settings...)
		selection.Extra = cloneRawMap(selection.Extra)
		copied.Columns[key] = selection
	}
	copied.Extra = cloneRawMap(entry.Extra)
	return copied
}

// cloneIntent copies apply intent slices and unknown fields.
func cloneIntent(intent *repository.ApplyIntent) *repository.ApplyIntent {
	if intent == nil {
		return nil
	}
	copied := *intent
	copied.Settings = append([]string{}, intent.Settings...)
	copied.Extra = cloneRawMap(intent.Extra)
	return &copied
}

// cloneRawMap copies JSONC extension values byte-for-byte.
func cloneRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	if values == nil {
		return map[string]json.RawMessage{}
	}
	copied := map[string]json.RawMessage{}
	for key, value := range values {
		copied[key] = append(json.RawMessage{}, value...)
	}
	return copied
}

// loadRenameSessions reads all valid PPID records without changing missing context files.
func loadRenameSessions(repo repository.Repository) ([]renameSession, error) {
	directory := filepath.Dir(repo.SessionPath(0))
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return []renameSession{}, nil
		}
		return nil, err
	}
	result := []renameSession{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		ppid, parseErr := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if parseErr != nil {
			continue
		}
		record, ok, loadErr := repo.LoadSession(ppid)
		if loadErr != nil {
			return nil, loadErr
		}
		if ok {
			result = append(result, renameSession{ppid: ppid, record: record})
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ppid < result[right].ppid })
	return result, nil
}

// historyChanged reports whether a history rewrite actually changes durable content.
func historyChanged(history []repository.HistoryEntry, project warehouse.Project, repo repository.Repository) bool {
	original, err := repo.LoadHistory(project.Name)
	if err != nil {
		return true
	}
	dataA, _ := json.Marshal(original)
	dataB, _ := json.Marshal(history)
	return string(dataA) != string(dataB)
}

// currentStateChanged reports whether a state rewrite actually changes durable content.
func currentStateChanged(state repository.CurrentState, project warehouse.Project, repo repository.Repository) bool {
	original, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return true
	}
	dataA, _ := json.Marshal(original)
	dataB, _ := json.Marshal(state)
	return string(dataA) != string(dataB)
}

// pathExists checks one filesystem object without following a symlink target.
func pathExists(path string) bool { _, err := os.Lstat(path); return err == nil }

// containsPath checks one exact normalized path in a rollback inventory.
func containsPath(paths []string, candidate string) bool {
	candidate = filepath.Clean(candidate)
	for _, path := range paths {
		if filepath.Clean(path) == candidate {
			return true
		}
	}
	return false
}
