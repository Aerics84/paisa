package liabilities

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"gorm.io/gorm"
)

func GetRepayment(db *gorm.DB) []posting.Posting {
	postings := query.Init(db).Like("Liabilities:%").Credit().All()
	postings = service.PopulateMarketPrice(db, postings)
	expenses := query.Init(db).Like("Expenses:Interest:%").All()
	postings = append(postings, expenses...)
	if postings == nil {
		return []posting.Posting{}
	}
	return postings
}
