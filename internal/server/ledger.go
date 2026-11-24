package server

import (
	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"gorm.io/gorm"
)

func GetLedger(db *gorm.DB) LedgerResponse {
	postings := query.Init(db).Desc().All()
	postings = service.PopulateMarketPrice(db, postings)
	postings = accounting.PopulateBalance(postings)
	accounting.SortDesc(postings)
	return LedgerResponse{Postings: postings}
}
