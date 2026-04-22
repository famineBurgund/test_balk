package httpapi

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

var (
	wagonAliases   = []string{"wagon_number", "wag_number", "num_vag", "nom_vag", "vag_num", "num", "wagon", "vagon"}
	invoiceAliases = []string{"invoice_number", "nom_nak", "invoice", "nakladnaya", "n_inv"}
	ppsAliases     = []string{"pps_number", "num_pam", "nom_pam", "pps", "pam_number"}
	filedAliases   = []string{"date_nach", "date_pod", "data_pod", "arrival_date", "filed_at"}
	cleanAliases   = []string{"date_end", "date_ub", "data_ub", "departure_date", "cleaned_at"}
	eventAliases   = []string{"oper_time", "operation_time", "date_oper", "event_time", "updated_at"}
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

func makeDislocRecord(item map[string]any) dislocRecord {
	payload, _ := json.Marshal(item)
	return dislocRecord{
		WagonNumber: findString(item, wagonAliases),
		InvoiceNum:  findString(item, invoiceAliases),
		EventTime:   firstTime(item, eventAliases, filedAliases, cleanAliases),
		Payload:     payload,
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

func firstTime(item map[string]any, aliasGroups ...[]string) *time.Time {
	for _, aliases := range aliasGroups {
		for _, alias := range aliases {
			for key, value := range item {
				if strings.EqualFold(key, alias) {
					if parsed := parseTimeCandidates(value); parsed != nil {
						return parsed
					}
				}
			}
		}
	}
	return nil
}

func parseTimeCandidates(value any) *time.Time {
	if typed, ok := value.(string); ok {
		return parseTimeString(typed)
	}
	return nil
}

func parseTimeString(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"02.01.2006 15:04:05",
		"02.01.2006 15:04",
		"02.01.2006",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			return &utc
		}
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func sourceFromEndpoint(endpoint string) string {
	switch endpoint {
	case "attis", "at_emd", "filed-cars-at", "pps-status-at":
		return "at"
	case "nmtp", "filed-cars-nmtp", "pps-status-nmtp":
		return "nmtp"
	case "ut_emd":
		return "ut"
	case "gut_emd":
		return "gut"
	default:
		return ""
	}
}
