package httpapi

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	minutes := 5
	if raw := r.URL.Query().Get("minutes"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			minutes = value
		}
	}
	file, err := os.Open(filepath.Join(h.cfg.LogDir, "service.log"))
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer file.Close()

	var logs []string
	threshold := time.Now().Add(-time.Duration(minutes) * time.Minute)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 19 {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", line[:19], time.Local)
		if err == nil && ts.After(threshold) {
			logs = append(logs, line)
		}
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"status": "success", "logs": logs})
}
