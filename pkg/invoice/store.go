package invoice

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Store struct {
	mu           sync.RWMutex
	invoices     map[string]*Invoice
	mutasis      []*MutasiRecord
	filePath     string
	stopExpireCh chan struct{}
}

// NewStore initializes a thread-safe invoice store with file persistence.
func NewStore(dataFile string) (*Store, error) {
	s := &Store{
		invoices:     make(map[string]*Invoice),
		mutasis:      make([]*MutasiRecord, 0),
		filePath:     dataFile,
		stopExpireCh: make(chan struct{}),
	}

	if err := s.loadFromFile(); err != nil && !os.IsNotExist(err) {
		// Log but do not fail hard if file is just empty
		fmt.Printf("[WARN] Failed to load data file %s: %v. Starting fresh.\n", dataFile, err)
	}

	// Start background cleanup ticker for expired invoices
	go s.expireWorker()

	return s, nil
}

// Close gracefully stops the background worker and persists data.
func (s *Store) Close() error {
	close(s.stopExpireCh)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveToFileLocked()
}

// Save creates or updates an invoice.
func (s *Store) Save(inv *Invoice) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.invoices[inv.ID] = inv
	return s.saveToFileLocked()
}

// Get retrieves an invoice by its unique ID.
func (s *Store) Get(id string) (*Invoice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inv, exists := s.invoices[id]
	if !exists {
		return nil, false
	}
	// Copy to avoid external mutation
	copyInv := *inv
	return &copyInv, true
}

// FindPendingByAmount finds a currently PENDING, unexpired invoice matching the given total amount.
func (s *Store) FindPendingByAmount(amount int64) (*Invoice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, inv := range s.invoices {
		if inv.Status == StatusPending && inv.TotalAmount == amount && inv.ExpiresAt.After(now) {
			copyInv := *inv
			return &copyInv, true
		}
	}
	return nil, false
}

// MarkPaid atomically marks an invoice as PAID, linking it to the mutasi record.
func (s *Store) MarkPaid(invoiceID string, mutasi *MutasiRecord) (*Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, exists := s.invoices[invoiceID]
	if !exists {
		return nil, fmt.Errorf("invoice %s not found", invoiceID)
	}

	if inv.Status == StatusPaid {
		return nil, errors.New("invoice already marked as paid (anti double-claim triggered)")
	}

	now := time.Now()
	inv.Status = StatusPaid
	inv.PaidAt = &now
	inv.PaymentData = map[string]interface{}{
		"source":        mutasi.Source,
		"mutasi_id":     mutasi.ID,
		"raw_message":   mutasi.RawMessage,
		"verified_time": now,
	}

	mutasi.MatchedInvoiceID = invoiceID
	s.mutasis = append(s.mutasis, mutasi)

	if err := s.saveToFileLocked(); err != nil {
		fmt.Printf("[ERROR] Failed to save after mark paid: %v\n", err)
	}

	copyInv := *inv
	return &copyInv, nil
}

// RecordUnmatchedMutasi records incoming funds that did not match any invoice for auditing.
func (s *Store) RecordUnmatchedMutasi(mutasi *MutasiRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mutasis = append(s.mutasis, mutasi)
	_ = s.saveToFileLocked()
}

// List returns the latest N invoices, sorted by creation date descending.
func (s *Store) List(limit int) []*Invoice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Invoice, 0, len(s.invoices))
	for _, inv := range s.invoices {
		c := *inv
		list = append(list, &c)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}

// GenerateUniqueAmount finds an unused unique code (1-999) for this base amount among active pending invoices.
func (s *Store) GenerateUniqueAmount(baseAmount int64) (int64, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	usedCodes := make(map[int]bool)
	for _, inv := range s.invoices {
		if inv.Status == StatusPending && inv.BaseAmount == baseAmount && inv.ExpiresAt.After(now) {
			usedCodes[inv.UniqueCode] = true
		}
	}

	// Try finding an unused random 3-digit code (1 to 999)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for attempts := 0; attempts < 100; attempts++ {
		code := rng.Intn(999) + 1
		if !usedCodes[code] {
			return baseAmount + int64(code), code
		}
	}

	// Fallback to sequential search
	for code := 1; code <= 999; code++ {
		if !usedCodes[code] {
			return baseAmount + int64(code), code
		}
	}

	return baseAmount, 0
}

func (s *Store) expireWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			changed := false
			for _, inv := range s.invoices {
				if inv.Status == StatusPending && now.After(inv.ExpiresAt) {
					inv.Status = StatusExpired
					changed = true
				}
			}
			if changed {
				_ = s.saveToFileLocked()
			}
			s.mu.Unlock()
		case <-s.stopExpireCh:
			return
		}
	}
}

type persistedData struct {
	Invoices map[string]*Invoice `json:"invoices"`
	Mutasis  []*MutasiRecord     `json:"mutasis"`
}

func (s *Store) saveToFileLocked() error {
	if s.filePath == "" {
		return nil
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data := persistedData{
		Invoices: s.invoices,
		Mutasis:  s.mutasis,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, bytes, 0600)
}

func (s *Store) loadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var data persistedData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	if data.Invoices != nil {
		s.invoices = data.Invoices
	}
	if data.Mutasis != nil {
		s.mutasis = data.Mutasis
	}

	return nil
}
