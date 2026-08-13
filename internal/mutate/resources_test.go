package mutate

import (
	"errors"
	"reflect"
	"testing"
)

// TestValidateCanonicalNameRejectsReservedAndUnsafeNames verifies shared canonical validation.
func TestValidateCanonicalNameRejectsReservedAndUnsafeNames(t *testing.T) {
	tests := []struct {
		kind ResourceKind
		name string
	}{
		{kind: ProjectKind, name: "global"},
		{kind: ProjectKind, name: "ProjectIndex.jsonc"},
		{kind: ColumnKind, name: "ColumnIndex.jsonc"},
		{kind: SettingKind, name: "SettingIndex.jsonc"},
		{kind: ModeKind, name: ".cfgfc-temp"},
		{kind: ProjectKind, name: "../outside"},
		{kind: ColumnKind, name: "a/b"},
		{kind: SettingKind, name: " spaced "},
		{kind: ModeKind, name: ""},
	}
	for _, test := range tests {
		if err := ValidateCanonicalName(test.kind, test.name); err == nil {
			t.Fatalf("ValidateCanonicalName(%q, %q) succeeded", test.kind, test.name)
		}
	}
}

// TestNormalizeAliasesRejectsEmptyDuplicateAndReservedValues verifies replacement validation.
func TestNormalizeAliasesRejectsEmptyDuplicateAndReservedValues(t *testing.T) {
	for _, aliases := range [][]string{{""}, {"alpha", "alpha"}, {"global"}} {
		if _, err := NormalizeAliases(ProjectKind, aliases); err == nil {
			t.Fatalf("NormalizeAliases(%#v) succeeded", aliases)
		}
	}
	aliases, err := NormalizeAliases(ColumnKind, []string{" alpha ", "beta"})
	if err != nil {
		t.Fatalf("NormalizeAliases: %v", err)
	}
	if !reflect.DeepEqual(aliases, []string{"alpha", "beta"}) {
		t.Fatalf("aliases = %#v", aliases)
	}
}

// TestValidateIdentityScopeRejectsCanonicalAndAliasCollisions verifies per-scope uniqueness.
func TestValidateIdentityScopeRejectsCanonicalAndAliasCollisions(t *testing.T) {
	existing := []Identity{{CanonicalName: "First", Aliases: []string{"one"}}, {CanonicalName: "Second", Aliases: []string{"two"}}}
	candidates := []Identity{
		{CanonicalName: "Third", Aliases: []string{"First"}},
		{CanonicalName: "one", Aliases: []string{}},
	}
	for _, candidate := range candidates {
		err := ValidateIdentityScope(ProjectKind, candidate, existing, "")
		var mutationErr *Error
		if !errors.As(err, &mutationErr) || mutationErr.Kind != ConflictError {
			t.Fatalf("collision error = %#v", err)
		}
	}
	if err := ValidateIdentityScope(ProjectKind, Identity{CanonicalName: "First", Aliases: []string{"renamed-alias"}}, existing, "First"); err != nil {
		t.Fatalf("replacement of same identity failed: %v", err)
	}
}
