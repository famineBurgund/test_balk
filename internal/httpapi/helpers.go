package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func appendJSONBatch(path string, payload []byte, keep int) error {
	var batches []json.RawMessage
	if data, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(data)) > 0 {
		_ = json.Unmarshal(data, &batches)
	}
	batches = append(batches, append([]byte(nil), payload...))
	if len(batches) > keep {
		batches = batches[len(batches)-keep:]
	}
	formatted, err := json.MarshalIndent(batches, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, formatted, 0o644)
}

func writeProxyResponse(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func normalizeWagonList(value any) []string {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	case []any:
		var out []string
		for _, item := range v {
			if str, ok := item.(string); ok && strings.TrimSpace(str) != "" {
				out = append(out, strings.TrimSpace(str))
			}
		}
		return out
	default:
		return nil
	}
}

func isTenDigits(value string) bool {
	return len(value) == 10 && strings.Trim(value, "0123456789") == ""
}

func isEightDigits(value string) bool {
	return len(value) == 8 && strings.Trim(value, "0123456789") == ""
}

func maxIntervalTo(payload []byte, current *time.Time) *time.Time {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return current
	}
	data, _ := body["data"].(map[string]any)
	intervals, _ := data["api_processed_intervals"].([]any)
	for _, item := range intervals {
		interval, _ := item.(map[string]any)
		candidate := parseTimeCandidates(interval["to"])
		if candidate == nil {
			continue
		}
		if current == nil || candidate.After(*current) {
			value := *candidate
			current = &value
		}
	}
	return current
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func dislocHistoricalPrefix(endpoint string) string {
	switch endpoint {
	case "attis":
		return "at_dis"
	case "nmtp":
		return "nmtp_dis"
	case "ut_emd":
		return "ut_emd_dis"
	case "gut_emd":
		return "gut_emd_dis"
	case "at_emd":
		return "at_emd_dis"
	default:
		return strings.ReplaceAll(endpoint, "-", "_")
	}
}

func sanitizeFilename(name string) string {
	return strings.NewReplacer("/", "_", "-", "_").Replace(name)
}
