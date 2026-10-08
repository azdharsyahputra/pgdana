package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qris-gateway-go/pkg/config"
	"qris-gateway-go/pkg/handler"
	"qris-gateway-go/pkg/invoice"
	"qris-gateway-go/pkg/webhook"
)

func main() {
	cfg := config.Load()

	fmt.Println(`
 ===================================================
   ⚡ QRIS Gateway & Mutasi Scraper Engine (Go)
   -------------------------------------------------
   Standalone • Multi-Wallet • Anti Double-Claim
 ===================================================`)

	// Initialize Invoice & Mutasi Store
	store, err := invoice.NewStore(cfg.DataFile)
	if err != nil {
		log.Fatalf("[FATAL] Gagal inisialisasi database invoice: %v", err)
	}
	defer store.Close()

	// Initialize Outbound Webhook Dispatcher
	dispatcher := webhook.NewDispatcher(cfg.ClientWebhookURL)

	// Initialize Handlers
	apiHandler := handler.NewAPIHandler(cfg, store, dispatcher)
	webHandler, err := handler.NewWebHandler(cfg, store, "web/template.html")
	if err != nil {
		log.Printf("[WARN] Gagal memuat web/template.html: %v (UI /pay/{id} dinonaktifkan)", err)
	}

	mux := http.NewServeMux()

	// Public Health
	mux.HandleFunc("GET /health", apiHandler.HandleHealth)
	mux.HandleFunc("GET /api/health", apiHandler.HandleHealth)

	// QRIS Generation & Status
	mux.HandleFunc("GET /api/qris/stream/{id}", apiHandler.HandleStreamInvoice)
	mux.HandleFunc("POST /api/qris/create", apiHandler.HandleCreateQRIS)
	mux.HandleFunc("GET /api/qris/create", apiHandler.HandleCreateQRIS)
	mux.HandleFunc("GET /api/qris/status/{id}", apiHandler.HandleGetStatus)
	mux.HandleFunc("GET /api/qris/qr/{id}", apiHandler.HandleGetQRPNG)

	// Incoming Mutasi Webhooks (from Tasker / MacroDroid / SMS / Scraper)
	mux.HandleFunc("/api/webhook/notification", apiHandler.HandleWebhookNotification)
	mux.HandleFunc("/api/webhook/mutasi", apiHandler.HandleWebhookMutasi)

	// Admin / Dashboard Invoices
	mux.HandleFunc("GET /api/invoices", apiHandler.HandleListInvoices)

	// Web Checkout Page
	if webHandler != nil {
		mux.HandleFunc("GET /pay/{id}", webHandler.HandlePay)
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(corsMiddleware(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // Disabled for long-lived Server-Sent Events (SSE)
		IdleTimeout:  60 * time.Second,
	}

	// Channel for graceful shutdown
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("[INFO] Server berjalan pada http://localhost:%s\n", cfg.Port)
		fmt.Printf("[INFO] Merchant Name       : %s\n", cfg.MerchantName)
		fmt.Printf("[INFO] Mode Kode Unik      : %t (3 digit acak 1-999)\n", cfg.UseUniqueCode)
		fmt.Printf("[INFO] Kadaluwarsa Invoice : %s\n", cfg.InvoiceExpiry)
		fmt.Printf("[INFO] Endpoint Checkout   : http://localhost:%s/pay/{id}\n", cfg.Port)
		fmt.Printf("[INFO] Endpoint Webhook Notif: POST http://localhost:%s/api/webhook/notification\n", cfg.Port)
		fmt.Println("---------------------------------------------------")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server error: %v", err)
		}
	}()

	<-stopCh
	fmt.Println("\n[INFO] Menutup server secara aman (graceful shutdown)...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server shutdown error: %v", err)
	}
	fmt.Println("[INFO] Server berhasil dimatikan dengan aman.")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		// Only log API and Pay endpoints
		if r.URL.Path != "/health" {
			log.Printf("[%s] %s (%s)", r.Method, r.URL.Path, duration)
		}
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Webhook-Secret, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
