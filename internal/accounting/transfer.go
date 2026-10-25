package accounting

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

// IsInternalTransfer reports whether the given postings form an internal
// transfer between asset accounts. A transaction qualifies when it has at
// least two legs, every leg belongs to Assets, and at least one leg matches
// one of the configured transfer account globs.
func IsInternalTransfer(postings []posting.Posting, globs []string) bool {
	if len(globs) == 0 || len(postings) < 2 {
		return false
	}

	matchedGlob := false
	for _, p := range postings {
		if !utils.IsSameOrParent(p.Account, "Assets") {
			return false
		}

		for _, glob := range globs {
			if glob == "" {
				continue
			}
			match, err := matchAccountGlob(glob, p.Account)
			if err != nil {
				log.Warn("Invalid transfer account glob used for filtering ", glob, err)
				continue
			}
			if match {
				matchedGlob = true
			}
		}
	}

	return matchedGlob
}

// FilterOutInternalTransfers removes every posting that belongs to a
// transaction classified as an internal asset transfer.
func FilterOutInternalTransfers(postings []posting.Posting, globs []string) []posting.Posting {
	if len(globs) == 0 {
		return postings
	}

	internal := make(map[string]bool)
	for transactionID, ps := range lo.GroupBy(postings, func(p posting.Posting) string {
		return p.TransactionID
	}) {
		if IsInternalTransfer(ps, globs) {
			internal[transactionID] = true
		}
	}

	return lo.Filter(postings, func(p posting.Posting, _ int) bool {
		return !internal[p.TransactionID]
	})
}
