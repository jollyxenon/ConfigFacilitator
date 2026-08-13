package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
)

// testHandler builds one handler over an isolated HOME and warehouse root.
func testHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, ".configfacilitator")
	handler := &Handler{
		dependencies: WebDependencies{HomeDir: home, Environment: map[string]string{}, OperatingSystem: "linux"},
		rootPath:     root,
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return handler, root
}

// seedWarehouse builds one Project with a full Mode and one file Setting.
func seedWarehouse(t *testing.T, root string) {
	t.Helper()
	repo := repository.New(root)
	if err := workflow.ProjectCreate(repo, "OpenCode", "OpenCode", "", []string{"oc"}); err != nil {
		t.Fatalf("project create: %v", err)
	}
	if err := mutate.CreateColumn(repo, "OpenCode", "Models", mutate.Metadata{}); err != nil {
		t.Fatalf("column create: %v", err)
	}
	if _, _, err := mutate.AddColumnTarget(repo, "OpenCode", "Models", mutate.TargetPosition{Dir: filepath.Join(filepath.Dir(root), "targets"), Name: "", DirMode: "fixed", NameMode: "setting"}, planner.PlanOptions{HomeDir: filepath.Dir(root), Env: map[string]string{}, OS: "linux"}); err != nil {
		t.Fatalf("column target: %v", err)
	}
	if err := mutate.CreateSetting(repo, "OpenCode", "Models", "Alpha.txt", "file", mutate.Metadata{}, contentSourceText("alpha")); err != nil {
		t.Fatalf("setting create: %v", err)
	}
	if err := mutate.CreateMode(repo, "OpenCode", "Max", mutate.Metadata{DisplayName: "Max"}); err != nil {
		t.Fatalf("mode create: %v", err)
	}
	if err := workflow.SetModeColumn(repo, "OpenCode", "Max", "Models", "cover", []string{"Alpha.txt"}, false, planner.PlanOptions{HomeDir: filepath.Dir(root), Env: map[string]string{}, OS: "linux"}); err != nil {
		t.Fatalf("mode column set: %v", err)
	}
	if err := workflow.ApplyMode(repo, "OpenCode", "Max", false, planner.PlanOptions{HomeDir: filepath.Dir(root), Env: map[string]string{}, OS: "linux"}); err != nil {
		t.Fatalf("apply mode: %v", err)
	}
}

// contentSourceText builds one exact-byte file source.
func contentSourceText(text string) content.Source {
	return content.Source{Mode: content.SourceBytes, Bytes: []byte(text)}
}

func post(t *testing.T, handler *Handler, path string, payload any) (*httptest.ResponseRecorder, responseEnvelope) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var envelope responseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return recorder, envelope
}

func TestSnapshotAndRevisionFlow(t *testing.T) {
	handler, root := testHandler(t)
	seedWarehouse(t, root)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("snapshot status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	var envelope responseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("snapshot data = %#v", envelope.Data)
	}
	revision, _ := data["revision"].(string)
	if revision == "" {
		t.Fatal("snapshot has no revision")
	}
	projects := data["projects"].(map[string]any)
	project := projects["OpenCode"].(map[string]any)
	if project["available"] != true {
		t.Fatalf("project not available: %#v", project)
	}
	current := project["current"].(map[string]any)
	relation := current["relation"].(map[string]any)
	if relation["kind"] != "following" || relation["originMode"] != "Max" {
		t.Fatalf("unexpected current relation: %#v", current)
	}

	// A mutating command with the current revision succeeds.
	_, envelope = post(t, handler, "/api/command", map[string]any{
		"command":  "current.replace",
		"revision": revision,
		"project":  "OpenCode",
		"columns":  map[string]any{},
	})
	if !envelope.OK {
		t.Fatalf("current.replace failed: %#v", envelope.Error)
	}

	// A stale revision conflicts.
	_, envelope = post(t, handler, "/api/command", map[string]any{
		"command":  "current.replace",
		"revision": revision,
		"project":  "OpenCode",
		"columns":  map[string]any{},
	})
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "revision_conflict" {
		t.Fatalf("stale revision = %#v", envelope)
	}
}

func TestCommandRejectsUnknownAndContentRead(t *testing.T) {
	handler, root := testHandler(t)
	seedWarehouse(t, root)

	_, envelope := post(t, handler, "/api/command", map[string]any{"command": "not.a.command", "revision": "any"})
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "command_not_supported" {
		t.Fatalf("unknown command = code:%s msg:%s", envelope.Error.Code, envelope.Error.Message)
	}

	_, envelope = post(t, handler, "/api/command", map[string]any{
		"command": "setting.content.read",
		"project": "OpenCode",
		"column":  "Models",
		"setting": "Alpha.txt",
	})
	if !envelope.OK {
		t.Fatalf("content read failed: code=%s message=%s", envelope.Error.Code, envelope.Error.Message)
	}
	result := envelope.Data.(map[string]any)
	details := result["details"].(map[string]any)
	if details["content"] != "alpha" || details["encoding"] != "utf8" {
		t.Fatalf("content read = %#v", details)
	}
}

func TestPreviewReportsPlanWithoutCommit(t *testing.T) {
	handler, root := testHandler(t)
	seedWarehouse(t, root)

	_, envelope := post(t, handler, "/api/preview", map[string]any{
		"command": "current.preview",
		"project": "OpenCode",
		"columns": map[string]any{
			"Models": map[string]any{"strategy": "cover", "settings": []string{"Alpha.txt"}},
		},
	})
	if !envelope.OK {
		t.Fatalf("preview failed: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	details := data["details"].(map[string]any)
	mappings := details["mappings"].([]any)
	if len(mappings) != 1 {
		t.Fatalf("preview mappings = %#v", mappings)
	}
	if len(details["errors"].([]any)) != 0 {
		t.Fatalf("preview errors = %#v", details["errors"])
	}
}

func TestStaticAssetsServed(t *testing.T) {
	handler, _ := testHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("index status = %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("<html")) {
		t.Fatalf("index body = %q", recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte("cfgfc")) {
		t.Fatalf("app.js status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
