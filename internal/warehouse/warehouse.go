package warehouse

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/pathvars"
)

const warehouseRootBootstrapFileName = ".cfgfc-root"

// Warehouse stores the parsed warehouse root and all discovered projects.
type Warehouse struct {
	RootPath         string
	ProjectIndexPath string
	ProjectIndex     index.ProjectIndex
	Projects         map[string]Project
}

// Project stores one project and its filesystem/index relationships.
type Project struct {
	Name             string
	WarehouseName    string
	Path             string
	Missing          bool
	Metadata         index.ProjectEntry
	ColumnDirPath    string
	ModeDirPath      string
	BackupDirPath    string
	ColumnIndexPath  string
	ModeIndexPath    string
	CurrentStatePath string
	HistoryLogPath   string
	ColumnIndex      index.ColumnIndex
	ModeIndex        index.ModeIndex
	Columns          map[string]Column
	Modes            map[string]Mode
}

// Column stores one column and its declared settings.
type Column struct {
	Name             string
	WarehouseName    string
	Path             string
	Missing          bool
	Metadata         index.ColumnEntry
	SettingIndexPath string
	SettingIndex     index.SettingIndex
	Settings         map[string]Setting
}

// Setting stores one setting entry and its discovered filesystem state.
type Setting struct {
	Name          string
	WarehouseName string
	Path          string
	Exists        bool
	Missing       bool
	IsDir         bool
	Metadata      index.SettingEntry
}

// Mode stores one mode entry and its preserved state markers.
type Mode struct {
	Name          string
	WarehouseName string
	Missing       bool
	Metadata      index.ModeEntry
}

// DefaultWarehouseRoot returns the warehouse root in the user's profile-backed
// config directory. On native Windows os.UserHomeDir resolves %USERPROFILE%, so
// the default becomes %USERPROFILE%/.configfacilitator; Unix-like platforms use
// the current user's home directory.
func DefaultWarehouseRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return defaultWarehouseRootForHome(homeDir), nil
}

// EffectiveWarehouseRoot returns the configured warehouse root override when
// present, otherwise it falls back to the default warehouse root.
func EffectiveWarehouseRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return effectiveWarehouseRootForHome(homeDir)
}

// SetEffectiveWarehouseRoot normalizes and persists the warehouse root override
// in the user-scoped bootstrap file outside the warehouse.
func SetEffectiveWarehouseRoot(rootPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return setEffectiveWarehouseRootForHome(homeDir, runtime.GOOS, rootPath, currentEnvironmentMap())
}

// defaultWarehouseRootForHome keeps default root joining testable without
// depending on the host OS-specific os.UserHomeDir implementation.
func defaultWarehouseRootForHome(homeDir string) string {
	return filepath.Join(homeDir, ".configfacilitator")
}

// effectiveWarehouseRootForHome resolves the persisted override for a specific
// home directory and falls back to that home's default warehouse root.
func effectiveWarehouseRootForHome(homeDir string) (string, error) {
	overridePath := bootstrapFilePathForHome(homeDir)
	overrideRoot, hasOverride, err := readWarehouseRootOverride(overridePath)
	if err != nil {
		return "", fmt.Errorf("read warehouse root bootstrap %q: %w", overridePath, err)
	}
	if hasOverride {
		return overrideRoot, nil
	}
	return defaultWarehouseRootForHome(homeDir), nil
}

// setEffectiveWarehouseRootForHome normalizes and persists a warehouse root
// override for the provided home directory and operating system.
func setEffectiveWarehouseRootForHome(homeDir string, operatingSystem string, rootPath string, env map[string]string) (string, error) {
	normalizedRoot, err := normalizeWarehouseRootPath(rootPath, homeDir, operatingSystem, env)
	if err != nil {
		return "", err
	}
	bootstrapPath := bootstrapFilePathForHome(homeDir)
	if err := os.WriteFile(bootstrapPath, []byte(normalizedRoot+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write warehouse root bootstrap %q: %w", bootstrapPath, err)
	}
	return normalizedRoot, nil
}

// bootstrapFilePathForHome returns the user-scoped bootstrap file path that
// stores the persisted effective warehouse root override.
func bootstrapFilePathForHome(homeDir string) string {
	return filepath.Join(homeDir, warehouseRootBootstrapFileName)
}

// readWarehouseRootOverride loads the persisted warehouse root override from
// the bootstrap file when one exists.
func readWarehouseRootOverride(bootstrapPath string) (string, bool, error) {
	data, err := os.ReadFile(bootstrapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	overrideRoot := strings.TrimSpace(string(data))
	if overrideRoot == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(overrideRoot) {
		return "", false, fmt.Errorf("bootstrap file must contain an absolute path")
	}
	return filepath.Clean(overrideRoot), true, nil
}

// normalizeWarehouseRootPath expands variables and returns one cleaned absolute
// warehouse root path ready to persist in the bootstrap file.
func normalizeWarehouseRootPath(rootPath string, homeDir string, operatingSystem string, env map[string]string) (string, error) {
	trimmedRoot := strings.TrimSpace(rootPath)
	if trimmedRoot == "" {
		return "", fmt.Errorf("warehouse root path cannot be empty")
	}
	expandedRoot, err := pathvars.Expand(trimmedRoot, pathvars.Options{HomeDir: homeDir, Env: env, OS: operatingSystem})
	if err != nil {
		return "", err
	}
	absoluteRoot, err := filepath.Abs(expandedRoot)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absoluteRoot), nil
}

// currentEnvironmentMap snapshots the current process environment for path
// variable expansion during warehouse root normalization.
func currentEnvironmentMap() map[string]string {
	environment := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			environment[parts[0]] = parts[1]
		}
	}
	return environment
}

// LoadWarehouse loads the warehouse root, project index, and project models.
func LoadWarehouse(rootPath string) (Warehouse, error) {
	warehouse := Warehouse{
		RootPath:         rootPath,
		ProjectIndexPath: filepath.Join(rootPath, "ProjectIndex.jsonc"),
		Projects:         map[string]Project{},
	}

	projectIndex, err := loadProjectIndex(warehouse.ProjectIndexPath)
	if err != nil {
		return Warehouse{}, err
	}
	warehouse.ProjectIndex = projectIndex

	projectDirs, err := listWarehouseRootDirectories(rootPath)
	if err != nil {
		return Warehouse{}, err
	}

	projectNames := unionStringKeys(mapKeys(projectDirs), mapKeys(projectIndex.Projects))
	for _, projectWarehouseName := range projectNames {
		project, err := loadProject(rootPath, projectWarehouseName, projectIndex.Projects[projectWarehouseName], projectDirs[projectWarehouseName])
		if err != nil {
			return Warehouse{}, err
		}
		if err := registerProject(warehouse.Projects, project); err != nil {
			return Warehouse{}, err
		}
	}
	if err := validateProjectScope(warehouse.Projects); err != nil {
		return Warehouse{}, err
	}

	return warehouse, nil
}

// loadProject loads one project model from the warehouse root.
func loadProject(rootPath string, projectWarehouseName string, entry index.ProjectEntry, dirPresent bool) (Project, error) {
	if entry.WarehouseName == "" {
		entry.WarehouseName = projectWarehouseName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = entry.WarehouseName
	}
	projectPath := filepath.Join(rootPath, entry.WarehouseName)
	project := Project{
		Name:             entry.WarehouseName,
		WarehouseName:    entry.WarehouseName,
		Path:             projectPath,
		Missing:          !dirPresent,
		Metadata:         entry,
		ColumnDirPath:    filepath.Join(projectPath, "Column"),
		ModeDirPath:      filepath.Join(projectPath, "Mode"),
		BackupDirPath:    filepath.Join(projectPath, "Backup"),
		ColumnIndexPath:  filepath.Join(projectPath, "Column", "ColumnIndex.jsonc"),
		ModeIndexPath:    filepath.Join(projectPath, "Mode", "ModeIndex.jsonc"),
		CurrentStatePath: filepath.Join(projectPath, "Backup", "current_state.json"),
		HistoryLogPath:   filepath.Join(projectPath, "Backup", "history.log"),
		Columns:          map[string]Column{},
		Modes:            map[string]Mode{},
	}

	columnIndex, err := loadColumnIndex(project.ColumnIndexPath)
	if err != nil {
		return Project{}, err
	}
	project.ColumnIndex = columnIndex

	modeIndex, err := loadModeIndex(project.ModeIndexPath)
	if err != nil {
		return Project{}, err
	}
	project.ModeIndex = modeIndex

	columnDirs, err := listSubdirectories(project.ColumnDirPath)
	if err != nil {
		return Project{}, err
	}

	columnNames := unionStringKeys(mapKeys(columnDirs), mapKeys(columnIndex.Columns))
	for _, columnWarehouseName := range columnNames {
		column, err := loadColumn(project.ColumnDirPath, columnWarehouseName, columnIndex.Columns[columnWarehouseName], columnDirs[columnWarehouseName])
		if err != nil {
			return Project{}, err
		}
		if err := registerColumn(project.Columns, column, project.Name); err != nil {
			return Project{}, err
		}
	}

	for _, modeWarehouseName := range mapKeys(modeIndex.Modes) {
		modeEntry := modeIndex.Modes[modeWarehouseName]
		if modeEntry.WarehouseName == "" {
			modeEntry.WarehouseName = modeWarehouseName
		}
		if modeEntry.DisplayName == "" {
			modeEntry.DisplayName = modeEntry.WarehouseName
		}
		mode := Mode{
			Name:          modeEntry.WarehouseName,
			WarehouseName: modeEntry.WarehouseName,
			Missing:       hasMissingMarker(modeEntry.Extra),
			Metadata:      modeEntry,
		}
		if err := registerMode(project.Modes, mode, project.Name); err != nil {
			return Project{}, err
		}
	}
	if err := validateProjectChildren(project); err != nil {
		return Project{}, err
	}

	return project, nil
}

// loadColumn loads one column model and its setting entries.
func loadColumn(columnRoot string, columnWarehouseName string, entry index.ColumnEntry, dirPresent bool) (Column, error) {
	if entry.WarehouseName == "" {
		entry.WarehouseName = columnWarehouseName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = entry.WarehouseName
	}
	columnPath := filepath.Join(columnRoot, entry.WarehouseName)
	column := Column{
		Name:             entry.WarehouseName,
		WarehouseName:    entry.WarehouseName,
		Path:             columnPath,
		Missing:          !dirPresent,
		Metadata:         entry,
		SettingIndexPath: filepath.Join(columnPath, "SettingIndex.jsonc"),
		Settings:         map[string]Setting{},
	}

	settingIndex, err := loadSettingIndex(column.SettingIndexPath)
	if err != nil {
		return Column{}, err
	}
	column.SettingIndex = settingIndex

	settingEntries, err := listSettingEntries(columnPath)
	if err != nil {
		return Column{}, err
	}

	settingNames := unionStringKeys(mapKeys(settingEntries), mapKeys(settingIndex.Settings))
	for _, settingWarehouseName := range settingNames {
		metadata := settingIndex.Settings[settingWarehouseName]
		if metadata.WarehouseName == "" {
			metadata.WarehouseName = settingWarehouseName
		}
		if metadata.DisplayName == "" {
			metadata.DisplayName = metadata.WarehouseName
		}
		discovered := settingEntries[settingWarehouseName]
		setting := Setting{
			Name:          metadata.WarehouseName,
			WarehouseName: metadata.WarehouseName,
			Path:          filepath.Join(columnPath, metadata.WarehouseName),
			Exists:        discovered.exists,
			Missing:       !discovered.exists || hasMissingMarker(metadata.Extra),
			IsDir:         discovered.isDir,
			Metadata:      metadata,
		}
		if err := registerSetting(column.Settings, setting, column.Name); err != nil {
			return Column{}, err
		}
	}
	if err := validateColumnScope(column); err != nil {
		return Column{}, err
	}

	return column, nil
}

// ResolveProject resolves one project by normalized identifier or alias.
func (warehouse Warehouse) ResolveProject(reference string) (Project, error) {
	project, err := resolveReference(reference, warehouse.Projects, func(project Project) []string {
		return entityReferences(project.Name, project.Metadata.Aliases)
	}, "project")
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

// ResolveColumn resolves one column by normalized identifier or alias.
func (project Project) ResolveColumn(reference string) (Column, error) {
	column, err := resolveReference(reference, project.Columns, func(column Column) []string {
		return entityReferences(column.Name, column.Metadata.Aliases)
	}, "column")
	if err != nil {
		return Column{}, fmt.Errorf("project %q: %w", project.Name, err)
	}
	return column, nil
}

// ResolveMode resolves one mode by normalized identifier or alias.
func (project Project) ResolveMode(reference string) (Mode, error) {
	mode, err := resolveReference(reference, project.Modes, func(mode Mode) []string {
		return entityReferences(mode.Name, mode.Metadata.Aliases)
	}, "mode")
	if err != nil {
		return Mode{}, fmt.Errorf("project %q: %w", project.Name, err)
	}
	return mode, nil
}

// ResolveSetting resolves one setting by normalized identifier or alias.
func (column Column) ResolveSetting(reference string) (Setting, error) {
	setting, err := resolveReference(reference, column.Settings, func(setting Setting) []string {
		return entityReferences(setting.Name, setting.Metadata.Aliases)
	}, "setting")
	if err != nil {
		return Setting{}, fmt.Errorf("column %q: %w", column.Name, err)
	}
	return setting, nil
}

func registerProject(projects map[string]Project, project Project) error {
	if _, exists := projects[project.Name]; exists {
		return fmt.Errorf("duplicate project identifier %q", project.Name)
	}
	projects[project.Name] = project
	return nil
}

func registerColumn(columns map[string]Column, column Column, projectName string) error {
	if _, exists := columns[column.Name]; exists {
		return fmt.Errorf("project %q: duplicate column identifier %q", projectName, column.Name)
	}
	columns[column.Name] = column
	return nil
}

func registerMode(modes map[string]Mode, mode Mode, projectName string) error {
	if _, exists := modes[mode.Name]; exists {
		return fmt.Errorf("project %q: duplicate mode identifier %q", projectName, mode.Name)
	}
	modes[mode.Name] = mode
	return nil
}

func registerSetting(settings map[string]Setting, setting Setting, columnName string) error {
	if _, exists := settings[setting.Name]; exists {
		return fmt.Errorf("column %q: duplicate setting identifier %q", columnName, setting.Name)
	}
	settings[setting.Name] = setting
	return nil
}

func resolveReference[T any](reference string, entities map[string]T, refs func(T) []string, kind string) (T, error) {
	var zero T
	if reference == "" {
		return zero, fmt.Errorf("%s reference cannot be empty", kind)
	}
	matches := []T{}
	for _, entity := range entities {
		for _, candidate := range refs(entity) {
			if candidate == reference {
				matches = append(matches, entity)
				break
			}
		}
	}
	switch len(matches) {
	case 0:
		return zero, fmt.Errorf("%s %q does not exist", kind, reference)
	case 1:
		return matches[0], nil
	default:
		return zero, fmt.Errorf("%s reference %q is ambiguous", kind, reference)
	}
}

func entityReferences(normalizedName string, aliases []string) []string {
	refs := []string{}
	seen := map[string]struct{}{}
	for _, value := range append([]string{normalizedName}, aliases...) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		refs = append(refs, trimmed)
	}
	return refs
}

func validateProjectScope(projects map[string]Project) error {
	for _, project := range projects {
		for _, reference := range entityReferences(project.Name, project.Metadata.Aliases) {
			if reference == "global" {
				return fmt.Errorf("project reference %q is reserved", reference)
			}
		}
	}
	return validateReferenceScope(projects, func(project Project) []string {
		return entityReferences(project.Name, project.Metadata.Aliases)
	}, "project")
}

func validateProjectChildren(project Project) error {
	if err := validateReferenceScope(project.Columns, func(column Column) []string {
		return entityReferences(column.Name, column.Metadata.Aliases)
	}, fmt.Sprintf("project %q column", project.Name)); err != nil {
		return err
	}
	if err := validateReferenceScope(project.Modes, func(mode Mode) []string {
		return entityReferences(mode.Name, mode.Metadata.Aliases)
	}, fmt.Sprintf("project %q mode", project.Name)); err != nil {
		return err
	}
	for _, column := range project.Columns {
		if err := validateColumnScope(column); err != nil {
			return err
		}
	}
	return nil
}

func validateColumnScope(column Column) error {
	return validateReferenceScope(column.Settings, func(setting Setting) []string {
		return entityReferences(setting.Name, setting.Metadata.Aliases)
	}, fmt.Sprintf("column %q setting", column.Name))
}

func validateReferenceScope[T any](entities map[string]T, refs func(T) []string, scope string) error {
	seen := map[string]string{}
	for key, entity := range entities {
		for _, reference := range refs(entity) {
			if existing, exists := seen[reference]; exists && existing != key {
				return fmt.Errorf("%s reference %q collides", scope, reference)
			}
			seen[reference] = key
		}
	}
	return nil
}

// loadProjectIndex loads the optional project index file.
func loadProjectIndex(path string) (index.ProjectIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ProjectIndex{Projects: map[string]index.ProjectEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ProjectIndex{}, err
	}
	return index.ParseProjectIndex(data)
}

// loadColumnIndex loads the optional column index file.
func loadColumnIndex(path string) (index.ColumnIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ColumnIndex{Columns: map[string]index.ColumnEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ColumnIndex{}, err
	}
	return index.ParseColumnIndex(data)
}

// loadSettingIndex loads the optional setting index file.
func loadSettingIndex(path string) (index.SettingIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.SettingIndex{Settings: map[string]index.SettingEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.SettingIndex{}, err
	}
	return index.ParseSettingIndex(data)
}

// loadModeIndex loads the optional mode index file.
func loadModeIndex(path string) (index.ModeIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ModeIndex{Modes: map[string]index.ModeEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ModeIndex{}, err
	}
	return index.ParseModeIndex(data)
}

type discoveredEntry struct {
	exists bool
	isDir  bool
}

func isReservedWarehouseRootDirectory(name string) bool {
	return name == ".cfgfc-session"
}

// listWarehouseRootDirectories lists direct child project directories at the warehouse root.
func listWarehouseRootDirectories(path string) (map[string]bool, error) {
	directories, err := listSubdirectories(path)
	if err != nil {
		return nil, err
	}

	filtered := map[string]bool{}
	for name, exists := range directories {
		if !isReservedWarehouseRootDirectory(name) {
			filtered[name] = exists
		}
	}
	return filtered, nil
}

// listSubdirectories lists direct child directories, returning an empty map when the path does not exist.
func listSubdirectories(path string) (map[string]bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}

	directories := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			directories[entry.Name()] = true
		}
	}
	return directories, nil
}

// listSettingEntries lists direct setting files/directories while excluding the setting index itself.
func listSettingEntries(path string) (map[string]discoveredEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]discoveredEntry{}, nil
		}
		return nil, err
	}

	settings := map[string]discoveredEntry{}
	for _, entry := range entries {
		if entry.Name() == "SettingIndex.jsonc" {
			continue
		}
		settings[entry.Name()] = discoveredEntry{exists: true, isDir: entry.IsDir()}
	}
	return settings, nil
}

// unionStringKeys returns the sorted union of two key slices.
func unionStringKeys(left []string, right []string) []string {
	keys := map[string]struct{}{}
	for _, key := range left {
		keys[key] = struct{}{}
	}
	for _, key := range right {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

// mapKeys returns the sorted keys of a map.
func mapKeys[V any](input map[string]V) []string {
	keys := map[string]struct{}{}
	for key := range input {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

// sortedKeys returns lexicographically sorted keys from a set map.
func sortedKeys(input map[string]struct{}) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// hasMissingMarker reports whether the extra-field map contains a truthy `missing` marker.
func hasMissingMarker(extra map[string]json.RawMessage) bool {
	raw, ok := extra["missing"]
	if !ok {
		return false
	}
	var missing bool
	if err := json.Unmarshal(raw, &missing); err != nil {
		return false
	}
	return missing
}
