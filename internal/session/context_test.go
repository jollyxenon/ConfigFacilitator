package session

import "testing"

func TestStorePersistsProjectByPPID(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Set(1234, "OpenCode"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	project, ok, err := store.Get(1234)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected stored project to exist")
	}
	if project != "OpenCode" {
		t.Fatalf("expected OpenCode, got %q", project)
	}

	_, ok, err = store.Get(5678)
	if err != nil {
		t.Fatalf("Get returned error for unknown ppid: %v", err)
	}
	if ok {
		t.Fatalf("expected unknown ppid not to resolve")
	}
}

func TestResolveProjectPrefersExplicitProject(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Set(1234, "OpenCode"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	project, fromContext, err := ResolveProject("ClaudeCode", 1234, store)
	if err != nil {
		t.Fatalf("ResolveProject returned error: %v", err)
	}
	if project != "ClaudeCode" {
		t.Fatalf("expected explicit project to win, got %q", project)
	}
	if fromContext {
		t.Fatalf("expected explicit project not to be marked as context-derived")
	}
}

func TestResolveProjectUsesStoredContextWhenExplicitMissing(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Set(2222, "OpenCode"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	project, fromContext, err := ResolveProject("", 2222, store)
	if err != nil {
		t.Fatalf("ResolveProject returned error: %v", err)
	}
	if project != "OpenCode" {
		t.Fatalf("expected context project, got %q", project)
	}
	if !fromContext {
		t.Fatalf("expected project to be marked as context-derived")
	}
}
