package disloccheck

import (
	"strings"
	"sync"
	"time"

	"dislocservice/internal/logger"
	"dislocservice/internal/parser/dislocjson"
	filestore "dislocservice/internal/repository/files"
)

type FileRepository interface {
	ReadLastDislocCheckRun() time.Time
	WriteLastDislocCheckRun(at time.Time) error
	LoadInvoiceState() map[string][]string
	WriteInvoiceState(state map[string][]string) error
	LatestDislocCandidatesSince(lastRun time.Time) (atLatest, nmtpLatest *filestore.DislocCandidate, err error)
	ReadDislocCandidate(path string) ([]byte, error)
}

type Service struct {
	files  FileRepository
	logger *logger.Logger
	mu     sync.Mutex
}

func NewService(files FileRepository, appLogger *logger.Logger) *Service {
	return &Service{files: files, logger: appLogger}
}

func (s *Service) Run() {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastRun := s.files.ReadLastDislocCheckRun()
	atLatest, nmtpLatest, err := s.files.LatestDislocCandidatesSince(lastRun)
	if err != nil {
		s.logger.Error("disloc check read dir failed: " + err.Error())
		return
	}

	invoiceMap := s.files.LoadInvoiceState()
	s.processCandidate(atLatest, "at_disloc", invoiceMap)
	s.processCandidate(nmtpLatest, "nmtp_disloc", invoiceMap)

	if err := s.files.WriteInvoiceState(invoiceMap); err != nil {
		s.logger.Warn("disloc check write invoice state failed: " + err.Error())
	}
	if err := s.files.WriteLastDislocCheckRun(time.Now().UTC()); err != nil {
		s.logger.Warn("disloc check write timestamp failed: " + err.Error())
	}
}

func (s *Service) processCandidate(item *filestore.DislocCandidate, source string, invoiceMap map[string][]string) {
	if item == nil {
		return
	}
	data, err := s.files.ReadDislocCandidate(item.Path)
	if err != nil {
		s.logger.Warn("disloc check read file failed: " + err.Error())
		return
	}

	records, err := dislocjson.ExtractRecordsFromPayload(data)
	if err != nil {
		s.logger.Warn("disloc check parse file failed: " + err.Error())
		return
	}

	current := map[string]string{}
	for _, record := range records {
		invoice, date := dislocjson.ParseInvoiceState(record)
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
