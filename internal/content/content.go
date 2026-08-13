// Package content implements bounded Setting source creation and content operations.
package content

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// Kind identifies the filesystem shape of one Setting source.
type Kind string

// Setting source kinds are regular files or directories.
const (
	KindFile      Kind = "file"
	KindDirectory Kind = "directory"
)

// SourceMode identifies how bytes or a tree are supplied to an operation.
type SourceMode string

// Source modes distinguish empty creation, exact bytes, and local imports.
const (
	SourceEmpty SourceMode = "empty"
	SourceBytes SourceMode = "bytes"
	SourcePath  SourceMode = "path"
)

// Source describes one exact content source without conflating empty bytes with no source.
type Source struct {
	Mode  SourceMode
	Bytes []byte
	Path  string
}

// Entry describes one regular file or directory below a Setting source.
type Entry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

// ErrorKind identifies a content failure that the CLI can classify.
type ErrorKind string

// Content error kinds separate invalid input, conflicts, missing content, refusal, and persistence.
const (
	InvalidError     ErrorKind = "invalid"
	ConflictError    ErrorKind = "conflict"
	MissingError     ErrorKind = "missing"
	RefusalError     ErrorKind = "refusal"
	PersistenceError ErrorKind = "persistence"
)

// Error is a typed bounded-content failure.
type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
	Cause   error
}

// Error returns the concise content failure message.
func (err *Error) Error() string { return err.Message }

// Unwrap returns the underlying filesystem failure when present.
func (err *Error) Unwrap() error { return err.Cause }

// StageCreation validates and stages a complete Setting source beside its final destination.
func StageCreation(parent string, kind Kind, source Source) (string, func(), error) {
	if err := validateKind(kind); err != nil {
		return "", nil, err
	}
	if err := validateCreationSource(kind, source); err != nil {
		return "", nil, err
	}
	if kind == KindFile {
		data := []byte{}
		permission := os.FileMode(0o644)
		if source.Mode == SourceBytes {
			data = append([]byte{}, source.Bytes...)
		}
		if source.Mode == SourcePath {
			var err error
			data, permission, err = readRegularSource(source.Path)
			if err != nil {
				return "", nil, err
			}
		}
		staged, err := stageRegularFile(parent, data, permission)
		if err != nil {
			return "", nil, persistence("stage_file", "stage Setting file content", err)
		}
		return staged, cleanupPath(staged), nil
	}

	permission := os.FileMode(0o755)
	if source.Mode == SourcePath {
		info, err := inspectImportRoot(source.Path, KindDirectory)
		if err != nil {
			return "", nil, err
		}
		if err := validateImportTree(source.Path); err != nil {
			return "", nil, err
		}
		permission = info.Mode().Perm()
	}
	staged, err := os.MkdirTemp(parent, ".cfgfc-tmp-content-*")
	if err != nil {
		return "", nil, persistence("stage_directory", "stage Setting directory content", err)
	}
	cleanup := cleanupPath(staged)
	if err := os.Chmod(staged, permission); err != nil {
		cleanup()
		return "", nil, persistence("stage_directory", "set staged directory permissions", err)
	}
	if source.Mode == SourcePath {
		if err := copyImportDirectory(source.Path, staged); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return staged, cleanup, nil
}

// ValidateRelativePath validates a cross-platform lexical relative path and rejects symlink components.
func ValidateRelativePath(root string, relative string, allowEmpty bool) (string, error) {
	if relative == "" {
		if !allowEmpty {
			return "", invalid("empty_content_path", "content path cannot be empty", nil)
		}
		return validateRootBoundary(root)
	}
	if strings.ContainsRune(relative, '\x00') {
		return "", invalid("invalid_content_path", "content path contains a NUL byte", nil)
	}
	if isCrossPlatformAbsolute(relative) {
		return "", invalid("absolute_content_path", fmt.Sprintf("content path %q must be relative", relative), nil)
	}
	normalized := strings.ReplaceAll(relative, `\`, "/")
	segments := strings.Split(normalized, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", invalid("unclean_content_path", fmt.Sprintf("content path %q must contain only normal segments", relative), nil)
		}
		for _, character := range segment {
			if unicode.IsControl(character) {
				return "", invalid("invalid_content_path", fmt.Sprintf("content path %q contains a control character", relative), nil)
			}
		}
	}
	if path.Clean(normalized) != normalized {
		return "", invalid("unclean_content_path", fmt.Sprintf("content path %q is not clean", relative), nil)
	}
	rootPath, err := validateRootBoundary(root)
	if err != nil {
		return "", err
	}
	candidate := rootPath
	existing := true
	for index, segment := range segments {
		candidate = filepath.Join(candidate, segment)
		if !existing {
			continue
		}
		info, statErr := os.Lstat(candidate)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				existing = false
				continue
			}
			return "", persistence("inspect_content_path", fmt.Sprintf("inspect content path %q", relative), statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", invalid("symlink_content_path", fmt.Sprintf("content path %q contains a symbolic-link component", relative), nil)
		}
		if index < len(segments)-1 && !info.IsDir() {
			return "", invalid("non_directory_component", fmt.Sprintf("content path %q crosses a non-directory component", relative), nil)
		}
	}
	resolved, err := filepath.Abs(candidate)
	if err != nil {
		return "", persistence("resolve_content_path", fmt.Sprintf("resolve content path %q", relative), err)
	}
	rel, err := filepath.Rel(rootPath, resolved)
	if err != nil {
		return "", invalid("escaped_content_path", fmt.Sprintf("content path %q cannot be bounded to its Setting", relative), err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
		return "", invalid("escaped_content_path", fmt.Sprintf("content path %q must remain below its Setting root", relative), nil)
	}
	return resolved, nil
}

// List returns supported Setting entries in lexical relative-path order.
func List(root string, kind Kind) ([]Entry, error) {
	if err := inspectSettingRoot(root, kind); err != nil {
		return nil, err
	}
	if kind == KindFile {
		info, err := os.Lstat(root)
		if err != nil {
			return nil, persistence("inspect_setting", "inspect file-backed Setting", err)
		}
		return []Entry{{Path: ".", Kind: "file", Size: info.Size()}}, nil
	}
	entries := []Entry{}
	if err := listDirectory(root, root, &entries); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(left int, right int) bool { return entries[left].Path < entries[right].Path })
	return entries, nil
}

// Read returns exact bytes from a file-backed Setting or one directory-backed regular file.
func Read(root string, kind Kind, relative *string) ([]byte, error) {
	if err := inspectSettingRoot(root, kind); err != nil {
		return nil, err
	}
	selected := root
	if kind == KindFile {
		if relative != nil {
			return nil, invalid("file_path_forbidden", "file-backed Setting read does not accept a relative path", nil)
		}
	} else {
		if relative == nil {
			return nil, invalid("directory_path_required", "directory-backed Setting read requires a relative path", nil)
		}
		var err error
		selected, err = ValidateRelativePath(root, *relative, false)
		if err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(selected)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, missing("content_not_found", fmt.Sprintf("content file %q does not exist", displayRelative(relative)), err)
		}
		return nil, persistence("read_content", "inspect content file", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, invalid("content_not_file", fmt.Sprintf("content path %q is not a regular file", displayRelative(relative)), nil)
	}
	data, err := os.ReadFile(selected)
	if err != nil {
		return nil, persistence("read_content", fmt.Sprintf("read content file %q", displayRelative(relative)), err)
	}
	return data, nil
}

// Write atomically creates or replaces one regular file inside a repository transaction.
func Write(repo repository.Repository, root string, kind Kind, relative *string, source Source) error {
	if err := inspectSettingRoot(root, kind); err != nil {
		return err
	}
	data, err := loadWriteSource(source)
	if err != nil {
		return err
	}
	destination := root
	parents := ""
	if kind == KindFile {
		if relative != nil {
			return invalid("file_path_forbidden", "file-backed Setting write does not accept a relative path", nil)
		}
	} else {
		if relative == nil {
			return invalid("directory_path_required", "directory-backed Setting write requires a relative path", nil)
		}
		destination, err = ValidateRelativePath(root, *relative, false)
		if err != nil {
			return err
		}
		if info, statErr := os.Lstat(destination); statErr == nil {
			if info.IsDir() {
				return conflict("content_destination_directory", fmt.Sprintf("content destination %q is a directory", *relative), nil)
			}
			if !info.Mode().IsRegular() {
				return invalid("content_destination_special", fmt.Sprintf("content destination %q is not a regular file", *relative), nil)
			}
		} else if !os.IsNotExist(statErr) {
			return persistence("inspect_content_destination", fmt.Sprintf("inspect content destination %q", *relative), statErr)
		}
		parents = firstMissingParent(root, filepath.Dir(destination))
	}
	paths := []string{destination}
	if parents != "" {
		paths = append(paths, parents)
	}
	if err := repo.WithMutation("setting-content-write", paths, func() error {
		return repository.WriteFileAtomic(destination, data, 0o644)
	}); err != nil {
		return persistence("content_write", "write Setting content", err)
	}
	return nil
}

// Mkdir creates one nested directory and missing parents for a directory-backed Setting.
func Mkdir(repo repository.Repository, root string, kind Kind, relative string) error {
	if err := requireDirectorySetting(root, kind); err != nil {
		return err
	}
	destination, err := ValidateRelativePath(root, relative, false)
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(destination); statErr == nil && !info.IsDir() {
		return conflict("content_destination_exists", fmt.Sprintf("content destination %q already exists and is not a directory", relative), nil)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return persistence("inspect_content_destination", fmt.Sprintf("inspect content destination %q", relative), statErr)
	}
	snapshot := destination
	if _, err := os.Lstat(destination); os.IsNotExist(err) {
		snapshot = firstMissingParent(root, destination)
	}
	if err := repo.WithMutation("setting-content-mkdir", []string{snapshot}, func() error {
		return os.MkdirAll(destination, 0o755)
	}); err != nil {
		return persistence("content_mkdir", fmt.Sprintf("create content directory %q", relative), err)
	}
	return nil
}

// Move relocates one regular file or directory inside a directory-backed Setting without overwriting.
func Move(repo repository.Repository, root string, kind Kind, oldRelative string, newRelative string) error {
	if err := requireDirectorySetting(root, kind); err != nil {
		return err
	}
	source, err := ValidateRelativePath(root, oldRelative, false)
	if err != nil {
		return err
	}
	destination, err := ValidateRelativePath(root, newRelative, false)
	if err != nil {
		return err
	}
	if source == destination {
		return conflict("content_destination_exists", "content move destination is the source", nil)
	}
	sourceLexical := strings.ReplaceAll(oldRelative, `\`, "/")
	destinationLexical := strings.ReplaceAll(newRelative, `\`, "/")
	if strings.HasPrefix(destinationLexical, sourceLexical+"/") {
		return invalid("content_move_descendant", "cannot move a directory into its own descendant", nil)
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return missing("content_not_found", fmt.Sprintf("content path %q does not exist", oldRelative), err)
		}
		return persistence("inspect_content_source", fmt.Sprintf("inspect content source %q", oldRelative), err)
	}
	if !sourceInfo.IsDir() && !sourceInfo.Mode().IsRegular() {
		return invalid("content_source_special", fmt.Sprintf("content source %q is not a regular file or directory", oldRelative), nil)
	}
	if _, err := os.Lstat(destination); err == nil {
		return conflict("content_destination_exists", fmt.Sprintf("content destination %q already exists", newRelative), nil)
	} else if !os.IsNotExist(err) {
		return persistence("inspect_content_destination", fmt.Sprintf("inspect content destination %q", newRelative), err)
	}
	paths := []string{source, destination}
	if parent := firstMissingParent(root, filepath.Dir(destination)); parent != "" {
		paths = append(paths, parent)
	}
	if err := repo.WithMutation("setting-content-move", paths, func() error {
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.Rename(source, destination)
	}); err != nil {
		return persistence("content_move", fmt.Sprintf("move content %q to %q", oldRelative, newRelative), err)
	}
	return nil
}

// Delete removes one regular file or directory tree only after explicit confirmation.
func Delete(repo repository.Repository, root string, kind Kind, relative string, yes bool) error {
	if !yes {
		return refusal("confirmation_required", "content deletion requires --yes", nil)
	}
	if err := requireDirectorySetting(root, kind); err != nil {
		return err
	}
	destination, err := ValidateRelativePath(root, relative, false)
	if err != nil {
		return err
	}
	info, err := os.Lstat(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return missing("content_not_found", fmt.Sprintf("content path %q does not exist", relative), err)
		}
		return persistence("inspect_content_destination", fmt.Sprintf("inspect content path %q", relative), err)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return invalid("content_destination_special", fmt.Sprintf("content path %q is not a regular file or directory", relative), nil)
	}
	if err := repo.WithMutation("setting-content-delete", []string{destination}, func() error {
		if info.IsDir() {
			return os.RemoveAll(destination)
		}
		return os.Remove(destination)
	}); err != nil {
		return persistence("content_delete", fmt.Sprintf("delete content path %q", relative), err)
	}
	return nil
}

// validateKind rejects unsupported Setting source shapes.
func validateKind(kind Kind) error {
	if kind != KindFile && kind != KindDirectory {
		return invalid("invalid_setting_kind", "setting kind must be file or directory", nil)
	}
	return nil
}

// validateCreationSource enforces valid source-kind combinations before staging.
func validateCreationSource(kind Kind, source Source) error {
	if source.Mode != SourceEmpty && source.Mode != SourceBytes && source.Mode != SourcePath {
		return invalid("invalid_content_source", "content source mode is invalid", nil)
	}
	if kind == KindDirectory && source.Mode == SourceBytes {
		return invalid("directory_bytes_forbidden", "--stdin and --text are valid only for file-backed Settings", nil)
	}
	if source.Mode == SourcePath && strings.TrimSpace(source.Path) == "" {
		return invalid("empty_import_path", "--from path cannot be empty", nil)
	}
	return nil
}

// inspectImportRoot validates the selected import root without following symbolic links.
func inspectImportRoot(source string, kind Kind) (os.FileInfo, error) {
	info, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, invalid("import_source_not_found", fmt.Sprintf("import source %q does not exist", source), err)
		}
		return nil, persistence("inspect_import_source", fmt.Sprintf("inspect import source %q", source), err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, invalid("import_symlink", fmt.Sprintf("import source %q is a symbolic link", source), nil)
	}
	if kind == KindFile && !info.Mode().IsRegular() {
		return nil, invalid("import_kind_mismatch", fmt.Sprintf("import source %q is not a regular file", source), nil)
	}
	if kind == KindDirectory && !info.IsDir() {
		return nil, invalid("import_kind_mismatch", fmt.Sprintf("import source %q is not a directory", source), nil)
	}
	return info, nil
}

// readRegularSource loads exact bytes only after proving the source is a regular non-symlink file.
func readRegularSource(source string) ([]byte, os.FileMode, error) {
	info, err := inspectImportRoot(source, KindFile)
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, 0, persistence("read_import_source", fmt.Sprintf("read import source %q", source), err)
	}
	return data, info.Mode().Perm(), nil
}

// validateImportTree recursively proves an import contains only directories and regular files.
func validateImportTree(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return persistence("read_import_directory", fmt.Sprintf("read import directory %q", root), err)
	}
	for _, entry := range entries {
		entryPath := filepath.Join(root, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			return persistence("inspect_import_entry", fmt.Sprintf("inspect import entry %q", entryPath), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return invalid("import_symlink", fmt.Sprintf("import entry %q is a symbolic link", entryPath), nil)
		}
		if info.IsDir() {
			if err := validateImportTree(entryPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return invalid("import_special_file", fmt.Sprintf("import entry %q is not a regular file or directory", entryPath), nil)
		}
	}
	return nil
}

// copyImportDirectory copies one previously validated regular directory tree into staging.
func copyImportDirectory(source string, destination string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return persistence("read_import_directory", fmt.Sprintf("read import directory %q", source), err)
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return persistence("inspect_import_entry", fmt.Sprintf("inspect import entry %q", sourcePath), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return invalid("import_symlink", fmt.Sprintf("import entry %q is a symbolic link", sourcePath), nil)
		}
		if info.IsDir() {
			if err := os.Mkdir(destinationPath, info.Mode().Perm()); err != nil {
				return persistence("copy_import_directory", fmt.Sprintf("create imported directory %q", destinationPath), err)
			}
			if err := copyImportDirectory(sourcePath, destinationPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return invalid("import_special_file", fmt.Sprintf("import entry %q is not a regular file or directory", sourcePath), nil)
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return persistence("read_import_source", fmt.Sprintf("read import source %q", sourcePath), err)
		}
		if err := repository.WriteFileAtomic(destinationPath, data, info.Mode().Perm()); err != nil {
			return persistence("copy_import_file", fmt.Sprintf("copy import file %q", sourcePath), err)
		}
	}
	return nil
}

// stageRegularFile writes one exact sibling staging file and flushes it before installation.
func stageRegularFile(parent string, data []byte, permission os.FileMode) (string, error) {
	file, err := os.CreateTemp(parent, ".cfgfc-tmp-content-*")
	if err != nil {
		return "", err
	}
	staged := file.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(staged)
		}
	}()
	if err := file.Chmod(permission.Perm()); err != nil {
		_ = file.Close()
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	keep = true
	return staged, nil
}

// cleanupPath returns an idempotent best-effort staging cleanup closure.
func cleanupPath(staged string) func() {
	return func() {
		info, err := os.Lstat(staged)
		if err != nil {
			return
		}
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			_ = os.RemoveAll(staged)
			return
		}
		_ = os.Remove(staged)
	}
}

// validateRootBoundary resolves a Setting root and rejects a symbolic-link root.
func validateRootBoundary(root string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", persistence("resolve_setting_root", "resolve Setting root", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return "", missing("setting_missing", fmt.Sprintf("Setting source %q does not exist", absolute), err)
		}
		return "", persistence("inspect_setting_root", fmt.Sprintf("inspect Setting source %q", absolute), err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", invalid("symlink_setting_root", "Setting source root cannot be a symbolic link", nil)
	}
	return absolute, nil
}

// isCrossPlatformAbsolute recognizes Unix, macOS, Windows drive, rooted, UNC, and device paths.
func isCrossPlatformAbsolute(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '/' || value[0] == '\\' || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return true
	}
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

// inspectSettingRoot proves the Setting root exists with its declared regular shape.
func inspectSettingRoot(root string, kind Kind) error {
	if err := validateKind(kind); err != nil {
		return err
	}
	absolute, err := validateRootBoundary(root)
	if err != nil {
		return err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return persistence("inspect_setting_root", "inspect Setting root", err)
	}
	if kind == KindFile && !info.Mode().IsRegular() {
		return invalid("setting_kind_mismatch", "file-backed Setting source is not a regular file", nil)
	}
	if kind == KindDirectory && !info.IsDir() {
		return invalid("setting_kind_mismatch", "directory-backed Setting source is not a directory", nil)
	}
	return nil
}

// requireDirectorySetting rejects directory-only operations on file-backed Settings.
func requireDirectorySetting(root string, kind Kind) error {
	if err := inspectSettingRoot(root, kind); err != nil {
		return err
	}
	if kind != KindDirectory {
		return invalid("directory_setting_required", "content operation requires a directory-backed Setting", nil)
	}
	return nil
}

// listDirectory recursively records regular files and directories without following links.
func listDirectory(root string, current string, result *[]Entry) error {
	entries, err := os.ReadDir(current)
	if err != nil {
		return persistence("list_content", fmt.Sprintf("list content directory %q", current), err)
	}
	for _, entry := range entries {
		entryPath := filepath.Join(current, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			return persistence("inspect_content_entry", fmt.Sprintf("inspect content entry %q", entryPath), err)
		}
		relative, err := filepath.Rel(root, entryPath)
		if err != nil {
			return invalid("escaped_content_path", fmt.Sprintf("content entry %q cannot be bounded", entryPath), err)
		}
		relative = filepath.ToSlash(relative)
		if info.Mode()&os.ModeSymlink != 0 {
			return invalid("symlink_content_path", fmt.Sprintf("content entry %q is a symbolic link", relative), nil)
		}
		if info.IsDir() {
			*result = append(*result, Entry{Path: relative, Kind: "directory", Size: 0})
			if err := listDirectory(root, entryPath, result); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return invalid("content_special_file", fmt.Sprintf("content entry %q is not a regular file or directory", relative), nil)
		}
		*result = append(*result, Entry{Path: relative, Kind: "file", Size: info.Size()})
	}
	return nil
}

// loadWriteSource validates one required bytes-or-file source and returns exact bytes.
func loadWriteSource(source Source) ([]byte, error) {
	switch source.Mode {
	case SourceBytes:
		return append([]byte{}, source.Bytes...), nil
	case SourcePath:
		data, _, err := readRegularSource(source.Path)
		return data, err
	default:
		return nil, invalid("content_source_required", "content write requires exactly one source", nil)
	}
}

// displayRelative returns a stable root marker for file-backed content diagnostics.
func displayRelative(relative *string) string {
	if relative == nil {
		return "."
	}
	return *relative
}

// invalid constructs an invalid content-data failure.
func invalid(code string, message string, cause error) *Error {
	return &Error{Kind: InvalidError, Code: code, Message: message, Cause: cause}
}

// conflict constructs an existing-resource content failure.
func conflict(code string, message string, cause error) *Error {
	return &Error{Kind: ConflictError, Code: code, Message: message, Cause: cause}
}

// missing constructs a missing-content failure.
func missing(code string, message string, cause error) *Error {
	return &Error{Kind: MissingError, Code: code, Message: message, Cause: cause}
}

// refusal constructs a destructive-confirmation failure.
func refusal(code string, message string, cause error) *Error {
	return &Error{Kind: RefusalError, Code: code, Message: message, Cause: cause}
}

// persistence constructs a filesystem or repository-transaction content failure.
func persistence(code string, message string, cause error) *Error {
	if cause == nil {
		cause = errors.New(message)
	}
	return &Error{Kind: PersistenceError, Code: code, Message: message + ": " + cause.Error(), Cause: cause}
}

// firstMissingParent returns the highest absent path whose parent is already present.
func firstMissingParent(root string, target string) string {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	missing := ""
	for current := target; current != root && current != filepath.Dir(current); current = filepath.Dir(current) {
		if _, err := os.Lstat(current); err != nil {
			if os.IsNotExist(err) {
				missing = current
				continue
			}
			return current
		}
		break
	}
	return missing
}
