package invoice

import (
	"time"
)

type Status string

const (
	StatusPending Status = "PENDING"
	StatusPaid    Status = "PAID"
	StatusExpired Status = "EXPIRED"
)

// Invoice represents a payment request with a generated dynamic QRIS.
type Invoice struct {
	ID           string                 `json:"id"`
	OrderID      string                 `json:"order_id"`
	BaseAmount   int64                  `json:"base_amount"`
	UniqueCode   int                    `json:"unique_code"`
	TotalAmount  int64                  `json:"total_amount"`
	QRISContent  string                 `json:"qris_content"`
	QRISBase64   string                 `json:"qris_base64,omitempty"`
	Status       Status                 `json:"status"`
	CustomerInfo string                 `json:"customer_info,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	PaidAt       *time.Time             `json:"paid_at,omitempty"`
	PaymentData  map[string]interface{} `json:"payment_data,omitempty"`
}

// MutasiRecord represents an incoming fund notification from an e-wallet / bank.
type MutasiRecord struct {
	ID               string    `json:"id"`
	Source           string    `json:"source"` // GOPAY, BCA, MANDIRI, DANA, SHOPEEPAY, etc.
	Amount           int64     `json:"amount"`
	RawMessage       string    `json:"raw_message"`
	ReceivedAt       time.Time `json:"received_at"`
	MatchedInvoiceID string    `json:"matched_invoice_id,omitempty"`
}
