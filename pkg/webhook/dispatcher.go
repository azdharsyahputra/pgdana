package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"qris-gateway-go/pkg/invoice"
)

type Dispatcher struct {
	clientWebhookURL string
	client           *http.Client
}

func NewDispatcher(webhookURL string) *Dispatcher {
	return &Dispatcher{
		clientWebhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type WebhookPayload struct {
	Event       string                 `json:"event"`
	InvoiceID   string                 `json:"invoice_id"`
	OrderID     string                 `json:"order_id"`
	BaseAmount  int64                  `json:"base_amount"`
	UniqueCode  int                    `json:"unique_code"`
	TotalAmount int64                  `json:"total_amount"`
	Status      string                 `json:"status"`
	PaidAt      *time.Time             `json:"paid_at,omitempty"`
	PaymentData map[string]interface{} `json:"payment_data,omitempty"`
}

// NotifyPaymentSuccess sends a webhook notification to the client's system asynchronously with retries.
func (d *Dispatcher) NotifyPaymentSuccess(inv *invoice.Invoice) {
	if d.clientWebhookURL == "" {
		return
	}

	payload := WebhookPayload{
		Event:       "payment.success",
		InvoiceID:   inv.ID,
		OrderID:     inv.OrderID,
		BaseAmount:  inv.BaseAmount,
		UniqueCode:  inv.UniqueCode,
		TotalAmount: inv.TotalAmount,
		Status:      string(inv.Status),
		PaidAt:      inv.PaidAt,
		PaymentData: inv.PaymentData,
	}

	go func() {
		body, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("[ERROR] Webhook marshal error: %v\n", err)
			return
		}

		maxRetries := 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			req, err := http.NewRequest("POST", d.clientWebhookURL, bytes.NewBuffer(body))
			if err != nil {
				fmt.Printf("[ERROR] Webhook create request error: %v\n", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "QRIS-Gateway-Go/1.0")

			resp, err := d.client.Do(req)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				fmt.Printf("[INFO] Client webhook delivered successfully for invoice %s\n", inv.ID)
				return
			}

			if resp != nil {
				resp.Body.Close()
			}

			fmt.Printf("[WARN] Client webhook attempt %d failed for invoice %s. Retrying...\n", attempt, inv.ID)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		fmt.Printf("[ERROR] Failed to deliver client webhook for invoice %s after %d attempts\n", inv.ID, maxRetries)
	}()
}
