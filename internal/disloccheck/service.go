package disloccheck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dislocservice/internal/config"
	"dislocservice/internal/logger"
)

type Service struct {
	cfg    config.Config
	logger *logger.Logger
	mu     sync.Mutex
}

func NewService(cfg config.Config, appLogger *logger.Logger) *Service {
	return &Service{cfg: cfg, logger: appLogger}
}

func (s *Service) Run() {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastRunPath := filepath.Join(s.cfg.StateDir, "disloc_check_last.txt")
	actualListPath := filepath.Join(s.cfg.StateDir, "actual_invoice_list.txt")
	lastRun := readTimestamp(lastRunPath)

	entries, err := os.ReadDir(s.cfg.DislocSourceDir)
	if err != nil {
		s.logger.Error("disloc check read dir failed: " + err.Error())
		return
	}

	type candidate struct {
		name string
		path string
		when time.Time
	}

	var atLatest, nmtpLatest *candidate
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
		item := &candidate{name: name, path: filepath.Join(s.cfg.DislocSourceDir, name), when: info.ModTime()}
		if strings.HasPrefix(name, "at_dis") && (atLatest == nil || item.when.After(atLatest.when)) {
			atLatest = item
		}
		if strings.HasPrefix(name, "nmtp_dis") && (nmtpLatest == nil || item.when.After(nmtpLatest.when)) {
			nmtpLatest = item
		}
	}

	invoiceMap := loadInvoiceState(actualListPath)
	process := func(item *candidate, source string) {
		if item == nil {
			return
		}
		data, err := os.ReadFile(item.path)
		if err != nil {
			s.logger.Warn("disloc check read file failed: " + err.Error())
			return
		}

		var root any
		if err := json.Unmarshal(cleanJSON(data), &root); err != nil {
			s.logger.Warn("disloc check parse file failed: " + err.Error())
			return
		}

		current := map[string]string{}
		for _, record := range extractRecords(root) {
			invoice := normalizeInvoiceNumber(findString(record, invoiceAliases))
			date := findString(record, filedAliases)
			if invoice == "" || strings.TrimSpace(date) == "" {
				continue
			}
			current[invoice] = date
			invoiceMap[invoice] = []string{invoice, date, source}
		}

		for key, parts := range invoiceMap {
			if len(parts) < 3 {
				continue
			}
			if parts[2] == source {
				if _, ok := current[key]; !ok {
					delete(invoiceMap, key)
				}
			}
		}
	}

	process(atLatest, "at_disloc")
	process(nmtpLatest, "nmtp_disloc")
	writeInvoiceState(actualListPath, invoiceMap)
	_ = os.WriteFile(lastRunPath, []byte(time.Now().UTC().Format(time.RFC3339)), 0o644)
}

func readTimestamp(path string) time.Time {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
		return ts
	}
	return time.Time{}
}

func loadInvoiceState(path string) map[string][]string {
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
			result[normalizeInvoiceNumber(parts[0])] = parts
		}
	}
	return result
}

func writeInvoiceState(path string, state map[string][]string) {
	keys := make([]string, 0, len(state))
	for key := range state {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, strings.Join(state[key], ","))
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
