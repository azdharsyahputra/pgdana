package webhook

import (
	"regexp"
	"strconv"
	"strings"
)

// ParsedNotification represents the extracted details from a push notification or SMS.
type ParsedNotification struct {
	Amount     int64  `json:"amount"`
	Source     string `json:"source"`
	RawMessage string `json:"raw_message"`
}

// ExtractAmount parses monetary amounts from common Indonesian banking & e-wallet notification text.
// Examples:
// - "Pembayaran QRIS Rp 50.123 berhasil diterima" -> 50123
// - "Dana masuk Rp50.000" -> 50000
// - "m-Transfer: CR 08/10 150.000,00" -> 150000
func ParseNotificationText(text string, fallbackSource string) *ParsedNotification {
	textClean := strings.TrimSpace(text)
	if textClean == "" {
		return nil
	}

	source := detectSource(textClean, fallbackSource)
	amount := extractAmount(textClean)

	if amount <= 0 {
		return nil
	}

	return &ParsedNotification{
		Amount:     amount,
		Source:     source,
		RawMessage: textClean,
	}
}

func detectSource(text string, fallback string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "gobiz") || strings.Contains(lower, "gopay") || strings.Contains(lower, "gojek"):
		return "GOPAY"
	case strings.Contains(lower, "bca") || strings.Contains(lower, "klikbca") || strings.Contains(lower, "m-transfer"):
		return "BCA"
	case strings.Contains(lower, "mandiri") || strings.Contains(lower, "livin"):
		return "MANDIRI"
	case strings.Contains(lower, "shopee") || strings.Contains(lower, "shopeepay"):
		return "SHOPEEPAY"
	case strings.Contains(lower, "dana bisnis") || strings.Contains(lower, "aplikasi dana") || strings.HasPrefix(lower, "dana:") || strings.Contains(lower, "dompet dana"):
		return "DANA"
	case strings.Contains(lower, "ovo"):
		return "OVO"
	case strings.Contains(lower, "linkaja"):
		return "LINKAJA"
	case strings.Contains(lower, "bri") || strings.Contains(lower, "brimo"):
		return "BRI"
	default:
		if fallback != "" {
			return strings.ToUpper(fallback)
		}
		return "QRIS"
	}
}

// Regex patterns to find amounts with or without 'Rp' prefix
var (
	rpPattern     = regexp.MustCompile(`(?i)rp\.?\s*([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{2})?|[0-9]+)`)
	crPattern     = regexp.MustCompile(`(?i)(?:CR|DB)\s+[0-9/]+\s+([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{2})?|[0-9]+)`)
	amountPattern = regexp.MustCompile(`(?i)(?:sebesar|nominal|masuk|terima)\s+(?:rp\.?\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{2})?|[0-9]+)`)
	plainNumber   = regexp.MustCompile(`\b([0-9]{4,9})\b`)
)

func extractAmount(text string) int64 {
	// 1. Try finding pattern starting with "Rp"
	if matches := rpPattern.FindStringSubmatch(text); len(matches) > 1 {
		if val := cleanAndParse(matches[1]); val > 0 {
			return val
		}
	}

	// 2. Try finding BCA / Bank CR format: "CR 08/10 150.000,00"
	if matches := crPattern.FindStringSubmatch(text); len(matches) > 1 {
		if val := cleanAndParse(matches[1]); val > 0 {
			return val
		}
	}

	// 3. Try finding pattern like "sebesar 50.000" or "dana masuk 50000"
	if matches := amountPattern.FindStringSubmatch(text); len(matches) > 1 {
		if val := cleanAndParse(matches[1]); val > 0 {
			return val
		}
	}

	// 4. Fallback: look for 4-9 digit number
	if matches := plainNumber.FindStringSubmatch(text); len(matches) > 1 {
		if val := cleanAndParse(matches[1]); val > 0 {
			return val
		}
	}

	return 0
}

func cleanAndParse(valStr string) int64 {
	// Strip commas and decimals: "50.000,00" -> "50.000"
	if idx := strings.Index(valStr, ","); idx != -1 {
		valStr = valStr[:idx]
	}
	// Remove thousands separator dots: "50.000" -> "50000"
	valStr = strings.ReplaceAll(valStr, ".", "")
	valStr = strings.ReplaceAll(valStr, " ", "")

	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
