package httpapi

import (
	"encoding/json"
	"time"
)

type wagonVisit struct {
	ID            int64           `json:"id"`
	WagonNumber   string          `json:"wagon_number"`
	Source        string          `json:"source"`
	InvoiceNum    string          `json:"invoice_number,omitempty"`
	PPSNumber     string          `json:"pps_number,omitempty"`
	FiledAt       *time.Time      `json:"filed_at,omitempty"`
	CleanedAt     *time.Time      `json:"cleaned_at,omitempty"`
	OpenedPayload json.RawMessage `json:"opened_payload,omitempty"`
	ClosedPayload json.RawMessage `json:"closed_payload,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type dislocRecord struct {
	WagonNumber string
	InvoiceNum  string
	EventTime   *time.Time
	Payload     json.RawMessage
}
