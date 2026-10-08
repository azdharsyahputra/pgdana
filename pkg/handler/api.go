package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qris-gateway-go/pkg/config"
	"qris-gateway-go/pkg/invoice"
	"qris-gateway-go/pkg/qris"
	"qris-gateway-go/pkg/webhook"
)

type APIHandler struct {
	cfg        *config.Config
	store      *invoice.Store
	dispatcher *webhook.Dispatcher
}

func NewAPIHandler(cfg *config.Config, store *invoice.Store, dispatcher *webhook.Dispatcher) *APIHandler {
	return &APIHandler{
		cfg:        cfg,
		store:      store,
		dispatcher: dispatcher,
	}
}

type CreateQRISRequest struct {
	Amount       int64  `json:"amount"`
	OrderID      string `json:"order_id"`
	CustomerInfo string `json:"customer_info,omitempty"`
}

type CreateQRISResponse struct {
	Success     bool             `json:"success"`
	Message     string           `json:"message,omitempty"`
	CheckoutURL string           `json:"checkout_url,omitempty"`
	Invoice     *invoice.Invoice `json:"invoice,omitempty"`
}

// HandleCreateQRIS creates an invoice with dynamic QRIS and optional unique code.
func (h *APIHandler) HandleCreateQRIS(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateAPIKey(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Autentikasi gagal: API Key tidak valid",
		})
		return
	}

	var req CreateQRISRequest
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			_ = json.Unmarshal(body, &req)
		}
	}

	// Also support GET query parameters: ?amount=50000&order_id=ORD-123
	if req.Amount <= 0 {
		if amtStr := r.URL.Query().Get("amount"); amtStr != "" {
			req.Amount, _ = strconv.ParseInt(amtStr, 10, 64)
		}
	}
	if req.OrderID == "" {
		req.OrderID = r.URL.Query().Get("order_id")
	}

	if req.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Nominal pembayaran (amount) wajib diisi dan harus lebih dari 0",
		})
		return
	}

	if req.OrderID == "" {
		req.OrderID = generateShortID("ORD")
	}

	// Calculate unique code if enabled
	useUnique := h.cfg.UseUniqueCode
	if q := r.URL.Query().Get("unique_code"); q == "false" || q == "0" {
		useUnique = false
	}
	if req.Amount == 1 {
		useUnique = false
	}

	totalAmount := req.Amount
	uniqueCode := 0
	if useUnique {
		totalAmount, uniqueCode = h.store.GenerateUniqueAmount(req.Amount)
	}

	// Generate Dynamic QRIS
	staticQR := h.cfg.QRISStatic
	if staticQR == "" {
		// Dummy fallback template for initial testing if user hasn't set QRIS_STATIC in .env yet
		staticQR = "00020101021126600014ID.GOJEK.WWW01189360091430000000010210G0208770620303UME5204541153033605802ID5912MerchantTest6007Jakarta6304"
	}

	dynamicQR, err := qris.GenerateDynamicQRIS(staticQR, totalAmount)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Gagal generate dynamic QRIS: %v", err),
		})
		return
	}

	qrBase64, err := qris.GenerateQRCodeBase64(dynamicQR, 320)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Gagal generate QR image: %v", err),
		})
		return
	}

	now := time.Now()
	invID := generateShortID("INV")
	inv := &invoice.Invoice{
		ID:           invID,
		OrderID:      req.OrderID,
		BaseAmount:   req.Amount,
		UniqueCode:   uniqueCode,
		TotalAmount:  totalAmount,
		QRISContent:  dynamicQR,
		QRISBase64:   qrBase64,
		Status:       invoice.StatusPending,
		CustomerInfo: req.CustomerInfo,
		CreatedAt:    now,
		ExpiresAt:    now.Add(h.cfg.InvoiceExpiry),
	}

	if err := h.store.Save(inv); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Gagal menyimpan invoice ke database",
		})
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	checkoutURL := fmt.Sprintf("%s://%s/pay/%s", scheme, r.Host, inv.ID)

	writeJSON(w, http.StatusOK, CreateQRISResponse{
		Success:     true,
		Message:     "QRIS dinamis berhasil dibuat",
		CheckoutURL: checkoutURL,
		Invoice:     inv,
	})
}

// HandleGetStatus checks payment status for a specific invoice ID.
func (h *APIHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}

	inv, exists := h.store.Get(id)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"message": "Invoice tidak ditemukan",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"invoice": inv,
		"paid":    inv.Status == invoice.StatusPaid,
	})
}

// HandleStreamInvoice provides real-time status updates via Server-Sent Events (SSE).
func (h *APIHandler) HandleStreamInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming tidak didukung pada koneksi ini", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	inv, exists := h.store.Get(id)
	if !exists {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", `{"message":"Invoice tidak ditemukan"}`)
		flusher.Flush()
		return
	}

	// If already paid or expired, send immediately and close
	if inv.Status == invoice.StatusPaid || inv.Status == invoice.StatusExpired {
		data, _ := json.Marshal(map[string]interface{}{
			"paid":    inv.Status == invoice.StatusPaid,
			"status":  inv.Status,
			"invoice": inv,
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Send initial status
	initialData, _ := json.Marshal(map[string]interface{}{
		"paid":    false,
		"status":  inv.Status,
		"invoice": inv,
	})
	fmt.Fprintf(w, "data: %s\n\n", initialData)
	flusher.Flush()

	// Subscribe to live updates
	ch, unsubscribe := h.store.Subscribe(id)
	defer unsubscribe()

	keepAliveTicker := time.NewTicker(15 * time.Second)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAliveTicker.C:
			// Send comment ping to prevent proxy/nginx timeout
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		case updatedInv, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(map[string]interface{}{
				"paid":    updatedInv.Status == invoice.StatusPaid,
				"status":  updatedInv.Status,
				"invoice": updatedInv,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if updatedInv.Status == invoice.StatusPaid || updatedInv.Status == invoice.StatusExpired {
				return
			}
		}
	}
}

// HandleGetQRPNG returns the raw PNG image of the QR Code.
func (h *APIHandler) HandleGetQRPNG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	id = strings.TrimSuffix(id, ".png")

	inv, exists := h.store.Get(id)
	if !exists {
		http.Error(w, "QRIS tidak ditemukan", http.StatusNotFound)
		return
	}

	pngBytes, err := qris.GenerateQRCodePNG(inv.QRISContent, 320)
	if err != nil {
		http.Error(w, "Gagal membuat gambar QR", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(pngBytes)
}

// HandleWebhookNotification receives push notification text forwarded from Tasker / MacroDroid / SMS.
func (h *APIHandler) HandleWebhookNotification(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateWebhook(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Autentikasi webhook gagal: Secret tidak cocok",
		})
		return
	}

	body, _ := io.ReadAll(r.Body)
	log.Printf("[WEBHOOK INCOMING] Raw Body: %s", string(body))

	// Payload can be either JSON or raw string or GET query
	type notifPayload struct {
		Text       string `json:"text"`
		Message    string `json:"message"`
		Source     string `json:"source"`
		Title      string `json:"title"`
		RawContent string `json:"raw_content"`
	}

	var p notifPayload
	fullText := string(body)
	fallbackSource := "QRIS"

	// Check GET query parameter ?text=...
	if queryText := r.URL.Query().Get("text"); queryText != "" && fullText == "" {
		fullText = queryText
	}
	if queryMsg := r.URL.Query().Get("message"); queryMsg != "" && fullText == "" {
		fullText = queryMsg
	}

	if fullText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Payload webhook kosong (isi body JSON atau query ?text=...)",
		})
		return
	}

	if err := json.Unmarshal(body, &p); err == nil {
		if p.Text != "" {
			fullText = p.Text
		} else if p.Message != "" {
			fullText = p.Message
		} else if p.RawContent != "" {
			fullText = p.RawContent
		}
		if p.Title != "" {
			fullText = p.Title + " " + fullText
		}
		if p.Source != "" {
			fallbackSource = p.Source
		}
	}

	parsed := webhook.ParseNotificationText(fullText, fallbackSource)
	if parsed == nil || parsed.Amount <= 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Tidak dapat mendeteksi nominal transaksi dari notifikasi",
			"raw":     fullText,
		})
		return
	}

	mutasi := &invoice.MutasiRecord{
		ID:         generateShortID("MUT"),
		Source:     parsed.Source,
		Amount:     parsed.Amount,
		RawMessage: parsed.RawMessage,
		ReceivedAt: time.Now(),
	}

	// Find matching pending invoice
	matchedInv, found := h.store.FindPendingByAmount(parsed.Amount)
	if !found {
		// Log as unmatched
		h.store.RecordUnmatchedMutasi(mutasi)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"matched": false,
			"message": fmt.Sprintf("Mutasi Rp %d diterima tetapi tidak ada invoice PENDING yang cocok", parsed.Amount),
			"mutasi":  mutasi,
		})
		return
	}

	// Mark invoice as PAID atomically
	paidInv, err := h.store.MarkPaid(matchedInv.ID, mutasi)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Dispatch outbound webhook to client application/bot
	h.dispatcher.NotifyPaymentSuccess(paidInv)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"matched": true,
		"message": fmt.Sprintf("Pembayaran lunas untuk invoice %s sebesar Rp %d", paidInv.ID, paidInv.TotalAmount),
		"invoice": paidInv,
	})
}

// HandleWebhookMutasi directly receives pre-parsed mutasi JSON (e.g. from custom scraper or bank mutation service).
func (h *APIHandler) HandleWebhookMutasi(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateWebhook(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Autentikasi webhook gagal: Secret tidak cocok",
		})
		return
	}

	type directMutasi struct {
		Amount     int64  `json:"amount"`
		Source     string `json:"source"`
		RawMessage string `json:"raw_message"`
	}

	var m directMutasi
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil || m.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Format mutasi JSON tidak valid",
		})
		return
	}

	mutasi := &invoice.MutasiRecord{
		ID:         generateShortID("MUT"),
		Source:     strings.ToUpper(m.Source),
		Amount:     m.Amount,
		RawMessage: m.RawMessage,
		ReceivedAt: time.Now(),
	}

	matchedInv, found := h.store.FindPendingByAmount(m.Amount)
	if !found {
		h.store.RecordUnmatchedMutasi(mutasi)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"matched": false,
			"message": "Mutasi dicatat, tidak ada invoice pending yang cocok",
		})
		return
	}

	paidInv, err := h.store.MarkPaid(matchedInv.ID, mutasi)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	h.dispatcher.NotifyPaymentSuccess(paidInv)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"matched": true,
		"invoice": paidInv,
	})
}

// HandleListInvoices lists recent invoices for administration / dashboard.
func (h *APIHandler) HandleListInvoices(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateAPIKey(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "API Key tidak valid",
		})
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	invoices := h.store.List(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"count":    len(invoices),
		"invoices": invoices,
	})
}

// HandleHealth returns server health status.
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"service":   "QRIS Gateway Go",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *APIHandler) authenticateAPIKey(r *http.Request) bool {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}
	return apiKey != "" && apiKey == h.cfg.APIKey
}

func (h *APIHandler) authenticateWebhook(r *http.Request) bool {
	secret := r.Header.Get("X-Webhook-Secret")
	if secret == "" {
		secret = r.URL.Query().Get("secret")
	}
	return secret != "" && secret == h.cfg.WebhookSecret
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func generateShortID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	timestamp := time.Now().Format("060102")
	return fmt.Sprintf("%s-%s-%s", prefix, timestamp, strings.ToUpper(hex.EncodeToString(b)))
}
