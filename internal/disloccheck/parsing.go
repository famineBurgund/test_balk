package disloccheck

import (
	"bytes"
	"encoding/json"
	"strings"
)

var (
	invoiceAliases = []string{"invoice_number", "nom_nak", "invoice", "nakladnaya", "n_inv"}
	filedAliases   = []string{"date_nach", "date_pod", "data_pod", "arrival_date", "filed_at"}
)

func cleanJSON(payload []byte) []byte {
	return bytes.Map(func(r rune) rune {
		if r >= 0 && r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, payload)
}

func extractRecords(root any) []map[string]any {
	var found []map[string]any
	walkJSON(root, func(value any) {
		if typed, ok := value.([]any); ok {
			candidate := make([]map[string]any, 0, len(typed))
			for _, item := range typed {
				if row, ok := item.(map[string]any); ok {
					candidate = append(candidate, row)
				}
			}
			if len(candidate) > len(found) {
				found = candidate
			}
		}
	})
	return found
}

func walkJSON(value any, visit func(any)) {
	visit(value)
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			walkJSON(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkJSON(child, visit)
		}
	}
}

func findString(item map[string]any, aliases []string) string {
	normalized := map[string]any{}
	for key, value := range item {
		normalized[strings.ToLower(key)] = value
	}
	for _, alias := range aliases {
		if value, ok := normalized[alias]; ok {
			switch typed := value.(type) {
			case string:
				return strings.TrimSpace(typed)
			case float64:
				return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(jsonNumber(typed), ".0"), "."))
			}
		}
	}
	return ""
}

func jsonNumber(value float64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func normalizeInvoiceNumber(invoice string) string {
	replacer := strings.NewReplacer(
		"A", "А", "B", "В", "E", "Е", "K", "К", "M", "М", "H", "Н",
		"O", "О", "P", "Р", "C", "С", "T", "Т", "X", "Х", "Y", "У",
	)
	return replacer.Replace(strings.ToUpper(strings.TrimSpace(invoice)))
}
