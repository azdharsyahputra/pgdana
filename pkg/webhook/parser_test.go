package webhook

import "testing"

func TestParseNotificationText(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedSource string
		expectedAmount int64
	}{
		{
			name:           "GoBiz QRIS Payment",
			input:          "GoBiz: Pembayaran QRIS Rp 50.123 berhasil diterima",
			expectedSource: "GOPAY",
			expectedAmount: 50123,
		},
		{
			name:           "BCA m-Transfer",
			input:          "m-Transfer: CR 08/10 150.000,00 DARI BUKALAPAK",
			expectedSource: "BCA",
			expectedAmount: 150000,
		},
		{
			name:           "Livin Mandiri QRIS",
			input:          "Livin: Dana masuk Rp25.500 dari transaksi QRIS",
			expectedSource: "MANDIRI",
			expectedAmount: 25500,
		},
		{
			name:           "DANA Bisnis",
			input:          "Kamu menerima pembayaran DANA Bisnis sebesar Rp75.000",
			expectedSource: "DANA",
			expectedAmount: 75000,
		},
		{
			name:           "ShopeePay Merchant",
			input:          "ShopeePay: Pembayaran sebesar Rp 100.999 telah masuk",
			expectedSource: "SHOPEEPAY",
			expectedAmount: 100999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ParseNotificationText(tt.input, "")
			if res == nil {
				t.Fatalf("expected non-nil result for input: %s", tt.input)
			}
			if res.Source != tt.expectedSource {
				t.Errorf("expected source %s, got %s", tt.expectedSource, res.Source)
			}
			if res.Amount != tt.expectedAmount {
				t.Errorf("expected amount %d, got %d", tt.expectedAmount, res.Amount)
			}
		})
	}
}
