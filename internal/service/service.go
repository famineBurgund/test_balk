package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"dislocservice/internal/config"
	"dislocservice/internal/logger"
	"dislocservice/internal/parser/dislocjson"
	filestore "dislocservice/internal/repository/files"
	pgrepo "dislocservice/internal/repository/postgres"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type FileRepository interface {
	SaveDislocPayload(endpoint string, payload []byte) error
	AppendInvoiceBatch(direction string, payload []byte, keep int) error
	ReadRecentLogs(minutes int) ([]string, error)
}

type PostgresRepository interface {
	ListInvoiceBatchPayloads(ctx context.Context, direction string) ([][]byte, error)
	InsertInvoiceBatch(ctx context.Context, direction string, payload []byte) error
	InsertDislocationRows(ctx context.Context, table, endpoint, source string, records []dislocjson.DislocRecord) error
	ApplyFiledCars(ctx context.Context, source string, records []dislocjson.WagonEvent) error
	ApplyPPSStatus(ctx context.Context, source string, records []dislocjson.WagonEvent) error
	ListLatestWagonVisits(ctx context.Context) ([]pgrepo.WagonVisit, error)
	ListFiledWagonVisits(ctx context.Context) ([]pgrepo.WagonVisit, error)
	ListCleanWagonVisits(ctx context.Context) ([]pgrepo.WagonVisit, error)
	ListWagonHistory(ctx context.Context, wagon string) ([]pgrepo.WagonVisit, error)
}

type Service struct {
	cfg    config.Config
	db     PostgresRepository
	files  FileRepository
	client HTTPClient
	logger *logger.Logger
}

func New(cfg config.Config, db PostgresRepository, files FileRepository, client HTTPClient, appLogger *logger.Logger) *Service {
	return &Service{
		cfg:    cfg,
		db:     db,
		files:  files,
		client: client,
		logger: appLogger,
	}
}

func (s *Service) LastInvoiceInterval(ctx context.Context, direction string) (*time.Time, error) {
	payloads, err := s.db.ListInvoiceBatchPayloads(ctx, direction)
	if err != nil {
		return nil, err
	}
	var latest *time.Time
	for _, payload := range payloads {
		latest = maxIntervalTo(payload, latest)
	}
	return latest, nil
}

func (s *Service) SaveInvoiceBatch(ctx context.Context, direction string, payload []byte) error {
	if err := s.db.InsertInvoiceBatch(ctx, direction, payload); err != nil {
		return err
	}
	if err := s.files.AppendInvoiceBatch(direction, payload, 50); err != nil {
		s.logger.Warn("save invoice batch file failed: " + err.Error())
	}
	return nil
}

func (s *Service) ProxyJSONPost(ctx context.Context, suffix string, payload any) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.BaseSOAPServer, "/")+suffix, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, responseBody, nil
}

func (s *Service) FetchDisloc(ctx context.Context, endpoint string) (int, []byte, error) {
	targetURL := strings.TrimRight(s.cfg.BaseSOAPServer, "/") + "/disloc/" + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, payload, nil
	}

	if err := s.files.SaveDislocPayload(endpoint, payload); err != nil {
		s.logger.Warn("save disloc file failed: " + err.Error())
	}
	if err := s.persistDislocPayload(ctx, endpoint, payload); err != nil {
		s.logger.Warn("persist disloc payload failed: " + err.Error())
	}
	return http.StatusOK, payload, nil
}

func (s *Service) ListLatestWagons(ctx context.Context) ([]pgrepo.WagonVisit, error) {
	return s.db.ListLatestWagonVisits(ctx)
}

func (s *Service) ListFiledWagons(ctx context.Context) ([]pgrepo.WagonVisit, error) {
	return s.db.ListFiledWagonVisits(ctx)
}

func (s *Service) ListCleanWagons(ctx context.Context) ([]pgrepo.WagonVisit, error) {
	return s.db.ListCleanWagonVisits(ctx)
}

func (s *Service) WagonHistory(ctx context.Context, wagon string) ([]pgrepo.WagonVisit, error) {
	return s.db.ListWagonHistory(ctx, wagon)
}

func (s *Service) RecentLogs(minutes int) ([]string, error) {
	return s.files.ReadRecentLogs(minutes)
}

func (s *Service) persistDislocPayload(ctx context.Context, endpoint string, payload []byte) error {
	records, err := dislocjson.ExtractRecordsFromPayload(payload)
	if err != nil {
		return err
	}
	source := dislocjson.SourceFromEndpoint(endpoint)
	switch endpoint {
	case "attis", "nmtp":
		items := make([]dislocjson.DislocRecord, 0, len(records))
		for _, record := range records {
			items = append(items, dislocjson.ParseDislocRecord(record))
		}
		return s.db.InsertDislocationRows(ctx, "dislocation_main_events", endpoint, source, items)
	case "ut_emd", "gut_emd", "at_emd":
		items := make([]dislocjson.DislocRecord, 0, len(records))
		for _, record := range records {
			items = append(items, dislocjson.ParseDislocRecord(record))
		}
		return s.db.InsertDislocationRows(ctx, "dislocation_emd_events", endpoint, source, items)
	case "filed-cars-at", "filed-cars-nmtp":
		items := make([]dislocjson.WagonEvent, 0, len(records))
		for _, record := range records {
			items = append(items, dislocjson.ParseWagonEvent(record))
		}
		return s.db.ApplyFiledCars(ctx, source, items)
	case "pps-status-at", "pps-status-nmtp":
		items := make([]dislocjson.WagonEvent, 0, len(records))
		for _, record := range records {
			items = append(items, dislocjson.ParseWagonEvent(record))
		}
		return s.db.ApplyPPSStatus(ctx, source, items)
	default:
		return nil
	}
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
		candidate := dislocjson.ParseTimeCandidates(interval["to"])
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

var _ FileRepository = (*filestore.Repository)(nil)
