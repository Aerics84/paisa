package server

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ananthakumaran/paisa/internal/ledger"
)

var (
	quotedAmountPattern = regexp.MustCompile(`(-?\d[\d.,]*)\s+"([^"]+)"|"([^"]+)"\s+(-?\d[\d.,]*)`)
	quotedSymbolPattern = regexp.MustCompile(`"([^"]+)"`)
)

func ruleCommodityFormatMissing(deps Deps) []error {
	if deps.Config.Get().LedgerCli != "ledger" {
		return nil
	}

	files, err := ledger.Cli().Files(deps.Config.JournalPath())
	if err != nil {
		return nil
	}

	return missingCommodityFormats(files)
}

// missingCommodityFormats reports quoted commodities whose posting amounts use
// more than five decimal places but have no `commodity`/`format` directive.
// Ledger can then read the decimal comma as a thousands separator and silently
// store a wrong quantity.
func missingCommodityFormats(files []string) []error {
	contents := make([]string, 0, len(files))
	formatted := map[string]bool{}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		contents = append(contents, string(content))
	}

	// First pass: collect commodity blocks with an explicit format subdirective.
	// The blocks may live in a file that is included after the postings, so this
	// must not depend on file order.
	for _, content := range contents {
		current := ""
		for _, raw := range strings.Split(content, "\n") {
			line := strings.TrimRight(raw, "\r")
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
				continue
			}

			if !isIndented(line) {
				fields := strings.Fields(trimmed)
				if len(fields) > 0 && strings.EqualFold(fields[0], "commodity") {
					current = commodityDirectiveSymbol(trimmed)
				} else {
					current = ""
				}
				continue
			}

			fields := strings.Fields(trimmed)
			if current != "" && len(fields) > 0 && strings.EqualFold(fields[0], "format") {
				formatted[current] = true
			}
		}
	}

	// Second pass: inspect posting amounts.
	reported := map[string]bool{}
	symbols := make([]string, 0)
	for _, content := range contents {
		for _, raw := range strings.Split(content, "\n") {
			line := strings.TrimRight(raw, "\r")
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") || !isIndented(line) {
				continue
			}

			body := stripPostingAnnotations(line)
			for _, match := range quotedAmountPattern.FindAllStringSubmatch(body, -1) {
				symbol, amount := "", ""
				if match[1] != "" {
					amount, symbol = match[1], match[2]
				} else {
					symbol, amount = match[3], match[4]
				}

				if decimalPlaces(amount) <= 5 || formatted[symbol] || reported[symbol] {
					continue
				}
				reported[symbol] = true
				symbols = append(symbols, symbol)
			}
		}
	}

	sort.Strings(symbols)
	errs := make([]error, 0, len(symbols))
	for _, symbol := range symbols {
		escaped := html.EscapeString(symbol)
		errs = append(errs, fmt.Errorf(`<b>%s</b> has quantities with more than five decimal places, but no commodity format directive. Ledger can read the decimal comma as a thousands separator.<br/>Add <code>commodity "%s"</code> with <code>format 1.000,00000 "%s"</code> to the journal.`, escaped, escaped, escaped))
	}

	return errs
}

func isIndented(line string) bool {
	return len(line) > len(strings.TrimLeft(line, " \t"))
}

func commodityDirectiveSymbol(line string) string {
	rest := strings.TrimSpace(line[len("commodity"):])
	if rest == "" {
		return ""
	}

	if match := quotedSymbolPattern.FindStringSubmatch(rest); len(match) > 0 {
		return match[1]
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	// Accept `commodity 1.000,00 EUR`; the symbol is the last field.
	if len(fields) > 1 && looksLikeAmount(fields[0]) {
		return fields[len(fields)-1]
	}
	return fields[0]
}

func looksLikeAmount(value string) bool {
	hasDigit := false
	for _, r := range value {
		if unicode.IsDigit(r) {
			hasDigit = true
			continue
		}
		if r != '.' && r != ',' && r != '+' && r != '-' {
			return false
		}
	}
	return hasDigit
}

// stripPostingAnnotations removes inline comments and lot/price annotations so
// only the posting amount is inspected.
func stripPostingAnnotations(line string) string {
	body := stripOutsideQuotes(line, func(r rune) bool { return r == ';' })
	body = stripOutsideQuotes(body, func(r rune) bool { return r == '@' })
	body = stripOutsideQuotes(body, func(r rune) bool { return r == '{' || r == '[' })
	return body
}

func stripOutsideQuotes(value string, stop func(rune) bool) string {
	inQuote := false
	for index, r := range value {
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && stop(r) {
			return value[:index]
		}
	}
	return value
}

func decimalPlaces(amount string) int {
	amount = strings.TrimLeft(amount, "+-")
	if index := strings.IndexAny(amount, "eE"); index >= 0 {
		amount = amount[:index]
	}

	index := strings.LastIndexAny(amount, ".,")
	if index < 0 {
		return 0
	}

	fraction := amount[index+1:]
	for _, r := range fraction {
		if r < '0' || r > '9' {
			return 0
		}
	}
	return len(fraction)
}
