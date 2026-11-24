package server

import (
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/posting"
)

// FileError is the wire shape of ledger.LedgerFileError. It mirrors the
// validation diagnostic on the editor and sheet endpoints without pulling the
// whole ledger package into the generated wire types.
type FileError struct {
	FileName string `json:"file_name,omitempty"`
	LineFrom uint64 `json:"line_from"`
	LineTo   uint64 `json:"line_to"`
	Error    string `json:"error"`
	Message  string `json:"message"`
}

// EditorFilesResponse is the JSON body returned by GET /api/editor/files.
type EditorFilesResponse struct {
	Files       []LedgerFile `json:"files"`
	Accounts    []string     `json:"accounts"`
	Payees      []string     `json:"payees"`
	Commodities []string     `json:"commodities"`
}

// EditorFileResponse is the JSON body returned by POST /api/editor/file and
// POST /api/editor/file/delete_backups. An invalid path returns only the
// message, so both fields are optional on the wire.
type EditorFileResponse struct {
	File    *LedgerFile `json:"file,omitempty"`
	Message string      `json:"message,omitempty"`
}

// EditorValidateResponse is the JSON body returned by POST /api/editor/validate.
type EditorValidateResponse struct {
	Errors []FileError `json:"errors"`
	Output string      `json:"output"`
}

// EditorSaveResponse is the JSON body returned by POST /api/editor/save. A
// failed save has a message and no file; a successful save has a file and no
// message.
type EditorSaveResponse struct {
	Errors  []FileError `json:"errors"`
	Saved   bool        `json:"saved"`
	File    *LedgerFile `json:"file,omitempty"`
	Message string      `json:"message,omitempty"`
}

// SheetFilesResponse is the JSON body returned by GET /api/sheets/files.
type SheetFilesResponse struct {
	Files    []SheetFile       `json:"files"`
	Postings []posting.Posting `json:"postings"`
}

// SheetFileResponse is the JSON body returned by POST /api/sheets/file and
// POST /api/sheets/file/delete_backups.
type SheetFileResponse struct {
	File    *SheetFile `json:"file,omitempty"`
	Message string     `json:"message,omitempty"`
}

// SheetSaveResponse is the JSON body returned by POST /api/sheets/save.
type SheetSaveResponse struct {
	Saved   bool       `json:"saved"`
	File    *SheetFile `json:"file,omitempty"`
	Message string     `json:"message,omitempty"`
}

// fileErrors converts ledger diagnostics into the wire DTO and always returns
// a non-nil slice so the response contains [] instead of null.
func fileErrors(errors []ledger.LedgerFileError) []FileError {
	converted := make([]FileError, 0, len(errors))
	for _, item := range errors {
		converted = append(converted, FileError{
			FileName: item.FileName,
			LineFrom: item.LineFrom,
			LineTo:   item.LineTo,
			Error:    item.Error,
			Message:  item.Message,
		})
	}
	return converted
}
