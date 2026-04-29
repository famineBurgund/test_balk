package files

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dislocservice/internal/config"
	"dislocservice/internal/parser/dislocjson"
)

type Repository struct {
	jsonDir           string
	invoiceStorageDir string
	dislocSourceDir   string
	logDir            string
	stateDir          string
}

func New(cfg config.Config) *Repository {
	return &Repository{
		jsonDir:           cfg.JSONSaveDir,
		invoiceStorageDir: cfg.InvoiceStorageDir,
		dislocSourceDir:   cfg.DislocSourceDir,
		logDir:            cfg.LogDir,
		stateDir:          cfg.StateDir,
	}
}

func (r *Repository) SaveDislocPayload(endpoint string, payload []byte) error {
	now := time.Now().UTC().Format("20060102_150405")
	latestPath := filepath.Join(r.jsonDir, sanitizeFilename(endpoint)+".json")
	if err := os.WriteFile(latestPath, payload, 0o644); err != nil {
		return err
	}
	if prefix := dislocjson.HistoricalPrefix(endpoint); prefix != "" {
		historyPath := filepath.Join(r.dislocSourceDir, fmt.Sprintf("%s_%s.json", prefix, now))
		if err := os.WriteFile(historyPath, payload, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) AppendInvoiceBatch(direction string, payload []byte, keep int) error {
	path := filepath.Join(r.invoiceStorageDir, fmt.Sprintf("last_interval_%s.json", direction))
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

func (r *Repository) ReadRecentLogs(minutes int) ([]string, error) {
	file, err := os.Open(filepath.Join(r.logDir, "service.log"))
	if err != nil {
		return nil, err
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
	return logs, scanner.Err()
}

func (r *Repository) ReadLastDislocCheckRun() time.Time {
	data, err := os.ReadFile(filepath.Join(r.stateDir, "disloc_check_last.txt"))
	if err != nil {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
		return ts
	}
	return time.Time{}
}

func (r *Repository) WriteLastDislocCheckRun(at time.Time) error {
	return os.WriteFile(filepath.Join(r.stateDir, "disloc_check_last.txt"), []byte(at.UTC().Format(time.RFC3339)), 0o644)
}

func (r *Repository) LoadInvoiceState() map[string][]string {
	path := filepath.Join(r.stateDir, "actual_invoice_list.txt")
	result := map[string][]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) > 0 {
			result[dislocjson.NormalizeInvoiceNumber(parts[0])] = parts
		}
	}
	return result
}

func (r *Repository) WriteInvoiceState(state map[string][]string) error {
	path := filepath.Join(r.stateDir, "actual_invoice_list.txt")
	keys := make([]string, 0, len(state))
	for key := range state {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, strings.Join(state[key], ","))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

type DislocCandidate struct {
	Name string
	Path string
	When time.Time
}

func (r *Repository) LatestDislocCandidatesSince(lastRun time.Time) (atLatest, nmtpLatest *DislocCandidate, err error) {
	entries, err := os.ReadDir(r.dislocSourceDir)
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "at_dis") && !strings.HasPrefix(name, "nmtp_dis") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().After(lastRun) {
			continue
		}
		item := &DislocCandidate{Name: name, Path: filepath.Join(r.dislocSourceDir, name), When: info.ModTime()}
		if strings.HasPrefix(name, "at_dis") && (atLatest == nil || item.When.After(atLatest.When)) {
			atLatest = item
		}
		if strings.HasPrefix(name, "nmtp_dis") && (nmtpLatest == nil || item.When.After(nmtpLatest.When)) {
			nmtpLatest = item
		}
	}
	return atLatest, nmtpLatest, nil
}

func (r *Repository) ReadDislocCandidate(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func sanitizeFilename(name string) string {
	return strings.NewReplacer("/", "_", "-", "_").Replace(name)
}
