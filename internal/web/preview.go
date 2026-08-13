package web

import (
	"net/http"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
)

// previewResult reports the planned mappings, blocking errors, and targets
// that would require --force-targets for one draft selection.
type previewResult struct {
	Mappings     []repository.Mapping `json:"mappings"`
	Errors       []string             `json:"errors"`
	ForceTargets []string             `json:"forceTargets"`
}

// modePreview plans a draft Mode column selection without committing.
func (handler *Handler) modePreview(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return http.StatusInternalServerError, commandResult{}, &errorBody{Code: "preview_failed", Message: err.Error()}
	}
	project, err := loaded.ResolveProject(payload.Project)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	mode, err := project.ResolveMode(payload.Mode)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	columns := modeColumnsOf(mode.Metadata.Columns)
	result := handler.planForPreview(project, columns, nil)
	return http.StatusOK, commandResult{Details: result}, nil
}

// currentPreview plans a draft Current column selection without committing.
func (handler *Handler) currentPreview(rootPath string, payload commandRequest) (int, commandResult, *errorBody) {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return http.StatusInternalServerError, commandResult{}, &errorBody{Code: "preview_failed", Message: err.Error()}
	}
	project, err := loaded.ResolveProject(payload.Project)
	if err != nil {
		return http.StatusNotFound, commandResult{}, &errorBody{Code: "resource_not_found", Message: err.Error()}
	}
	state, err := repository.New(rootPath).LoadCurrentState(project.Name)
	if err != nil {
		return http.StatusUnprocessableEntity, commandResult{}, &errorBody{Code: "invalid", Message: err.Error()}
	}
	columns := indexColumnsOf(state.Columns)
	if len(payload.Columns) > 0 {
		columns = indexColumnsOf(payload.Columns)
	}
	result := handler.planForPreview(project, columns, state.Mappings)
	return http.StatusOK, commandResult{Details: result}, nil
}

// planForPreview runs the planner and reports unsafe targets that need force.
func (handler *Handler) planForPreview(project warehouse.Project, columns map[string]index.ModeColumnSelection, current []repository.Mapping) previewResult {
	result := previewResult{Errors: []string{}, ForceTargets: []string{}}
	mappings, err := planner.PlanColumns(project, columns, current, planOptions(handler.dependencies))
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	result.Mappings = mappings
	engine := linker.New()
	for _, mapping := range mappings {
		ownership, inspectErr := engine.InspectOwnership(mapping)
		if inspectErr == nil && ownership == linker.OwnershipUnmanaged {
			result.ForceTargets = append(result.ForceTargets, mapping.Target)
		}
	}
	return result
}

// modeColumnsOf converts persisted Mode selections to planner selections.
func modeColumnsOf(columns map[string]index.ModeColumnSelection) map[string]index.ModeColumnSelection {
	return columns
}

// indexColumnsOf converts persisted Current columns to planner selections.
func indexColumnsOf(columns map[string]repository.ColumnSelection) map[string]index.ModeColumnSelection {
	return workflow.ColumnsOf(columns)
}
