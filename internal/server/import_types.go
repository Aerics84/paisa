package server

import (
	"github.com/ananthakumaran/paisa/internal/importer"
	"github.com/ananthakumaran/paisa/internal/importer/reconciliation"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/ananthakumaran/paisa/internal/model/template"
)

// ParseStatementResponse is the JSON body returned by
// POST /api/import/statement. The embedded ingestion result keeps every staged
// transaction field on the wire while the generated TypeScript type extends
// IngestionResult.
type ParseStatementResponse struct {
	importer.IngestionResult `tstype:",extends"`
}

// ImportValidationError is a ledger validation error reported alongside a
// successful import commit. It mirrors ledger.LedgerFileError without pulling
// the whole ledger package into the generated wire types.
type ImportValidationError struct {
	FileName string `json:"file_name,omitempty"`
	LineFrom uint64 `json:"line_from"`
	LineTo   uint64 `json:"line_to"`
	Error    string `json:"error"`
	Message  string `json:"message"`
}

// CommitImportResponse is the JSON body returned by POST /api/import/commit on
// success. Error responses keep their existing {"error": ...} shape.
type CommitImportResponse struct {
	Success           bool                    `json:"success"`
	CommittedCount    int                     `json:"committed_count"`
	TargetFile        string                  `json:"target_file"`
	ManifestCommitted bool                    `json:"manifest_committed"`
	Errors            []ImportValidationError `json:"errors"`
}

// ReconciliationResponse is the JSON body returned by
// GET /api/import/reconciliation.
type ReconciliationResponse struct {
	reconciliation.ReconciliationReport `tstype:",extends"`
}

// RulesResponse is the JSON body returned by GET /api/rules.
type RulesResponse struct {
	Rules []import_rule.Rule `json:"rules"`
}

// SaveRuleResponse is the JSON body returned by POST /api/rules on success.
type SaveRuleResponse struct {
	Rule  import_rule.Rule `json:"rule"`
	Saved bool             `json:"saved"`
}

// DeleteRuleResponse is the JSON body returned by DELETE /api/rules/:id on
// success.
type DeleteRuleResponse struct {
	Success bool `json:"success"`
}

// TemplatesResponse is the JSON body returned by GET /api/templates.
type TemplatesResponse struct {
	Templates []template.Template `json:"templates"`
}

// UpsertTemplateResponse is the JSON body returned by
// POST /api/templates/upsert on success.
type UpsertTemplateResponse struct {
	Template template.Template `json:"template"`
	Saved    bool              `json:"saved"`
}

// DeleteTemplateResponse is the JSON body returned by
// POST /api/templates/delete on success.
type DeleteTemplateResponse struct {
	Success bool `json:"success"`
}
