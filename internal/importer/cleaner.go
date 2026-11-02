package importer

import (
	"regexp"
	"strings"
)

var (
	txPrefixRe   = regexp.MustCompile(`(?i)\b(UPI|NEFT|IMPS|RTGS|POS|ATM|ACH|BIL|MMT|BBPS|ECOM|VPS|IPS|PCD)[-/\s:*]+`)
	timeRe       = regexp.MustCompile(`(?i)\b\d{1,2}:\d{2}(:\d{2})?\b`)
	delimsRe     = regexp.MustCompile(`[-/|*:_]+|\s+`)
	multiSpaceRe = regexp.MustCompile(`\s+`)
	digitsOnlyRe = regexp.MustCompile(`^\d+$`)
	refCodeRe    = regexp.MustCompile(`(?i)^(REF|TXN|TRN|UTR|RRN|CHQ|AUTH|CRN|ID|NUM|NO|BEN)[A-Z0-9]*$`)
	fourDigitsRe = regexp.MustCompile(`\d{4,}`)
)

func isNoiseToken(w string) bool {
	if strings.Contains(w, "@") {
		return true
	}
	upper := strings.ToUpper(w)
	switch upper {
	case "UPI", "NEFT", "IMPS", "RTGS", "POS", "ATM", "ACH", "BIL", "MMT", "BBPS", "ECOM", "VPS", "IPS", "PCD", "PHP",
		"DR", "CR", "IN", "IND", "PVT", "LTD", "BANK", "S",
		"BKID", "JIOP", "PYTM", "YESB", "SBIN", "HDFC", "ICIC", "UTIB", "BARB", "CNRB", "PUNB", "UBIN", "KKBK":
		return true
	}
	// Tokens made up only of punctuation carry no payee information.
	if !strings.ContainsFunc(w, func(r rune) bool {
		return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}) {
		return true
	}
	// Pure digits of any length (dates, amounts, RRN, card numbers, times)
	if digitsOnlyRe.MatchString(w) {
		return true
	}
	// Reference prefixes like REF123, TXN123, BEN251026
	if refCodeRe.MatchString(w) {
		return true
	}
	// Alphanumeric tokens containing 4+ consecutive digits (e.g., AXB20261011..., PPPL31192011...)
	if fourDigitsRe.MatchString(w) {
		return true
	}
	// Long hex-like / hash tokens >= 10 chars containing both digits and letters (e.g., UPI5ca7522..., RBA01a128edf..., ICIf950ce...)
	if len(w) >= 10 {
		hasDigit := false
		hasLetter := false
		for _, r := range w {
			if r >= '0' && r <= '9' {
				hasDigit = true
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasLetter = true
			}
			if hasDigit && hasLetter {
				return true
			}
		}
	}
	return false
}

// CleanPayee extracts a concise merchant or payee name from raw bank statement narrations,
// stripping payment prefixes, 12-digit RRNs, gateway transaction IDs, and VPA handles.
func CleanPayee(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// In bank statements, strings often have multiple segments separated by '_'
	// e.g., UPI-628488132438-AXB20261011065534179345543681884180_UPI-628488132438-Smart Ba
	// The merchant name is usually in the latter part.
	parts := strings.Split(raw, "_")
	var bestCandidate string

	// Traverse parts in reverse so the merchant segment after '_' is prioritized
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		// Strip timestamps (e.g. 23:46) and payment system prefixes (UPI-, NEFT/, POS etc.)
		cleaned := timeRe.ReplaceAllString(part, " ")
		cleaned = txPrefixRe.ReplaceAllString(cleaned, " ")

		tokens := delimsRe.Split(cleaned, -1)
		var meaningful []string
		for _, t := range tokens {
			t = strings.TrimSpace(t)
			if t == "" || isNoiseToken(t) {
				continue
			}
			meaningful = append(meaningful, t)
		}

		if len(meaningful) > 0 {
			candidate := strings.Join(meaningful, " ")
			if len(candidate) > len(bestCandidate) {
				bestCandidate = candidate
			}
		}
	}

	if bestCandidate != "" {
		if len(bestCandidate) > 36 {
			bestCandidate = strings.TrimSpace(bestCandidate[:36])
		}
		return bestCandidate
	}

	// Fallback to basic clean if no pure words were found
	cleaned := txPrefixRe.ReplaceAllString(raw, "")
	cleaned = strings.TrimSpace(multiSpaceRe.ReplaceAllString(cleaned, " "))
	if len(cleaned) > 36 {
		cleaned = strings.TrimSpace(cleaned[:36])
	}
	return cleaned
}
