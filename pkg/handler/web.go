package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"qris-gateway-go/pkg/config"
	"qris-gateway-go/pkg/invoice"
)

type WebHandler struct {
	cfg   *config.Config
	store *invoice.Store
	tmpl  *template.Template
}

func NewWebHandler(cfg *config.Config, store *invoice.Store, tmplPath string) (*WebHandler, error) {
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse web template: %w", err)
	}

	return &WebHandler{
		cfg:   cfg,
		store: store,
		tmpl:  tmpl,
	}, nil
}

type checkoutViewModel struct {
	MerchantName    string
	FormattedAmount string
	Invoice         *invoice.Invoice
}

func (h *WebHandler) HandlePay(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		// Fallback for older router or query
		id = strings.TrimPrefix(r.URL.Path, "/pay/")
	}

	inv, exists := h.store.Get(id)
	if !exists {
		http.Error(w, "Invoice pembayaran tidak ditemukan", http.StatusNotFound)
		return
	}

	vm := checkoutViewModel{
		MerchantName:    h.cfg.MerchantName,
		FormattedAmount: formatRupiah(inv.TotalAmount),
		Invoice:         inv,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.Execute(w, vm)
}

func formatRupiah(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var res []string
	for len(s) > 3 {
		res = append([]string{s[len(s)-3:]}, res...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		res = append([]string{s}, res...)
	}
	return strings.Join(res, ".")
}
