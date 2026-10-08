package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all environment configurations for the gateway.
type Config struct {
	Port                 string
	APIKey               string
	WebhookSecret        string
	ClientWebhookURL     string
	QRISStatic           string
	MerchantName         string
	UseUniqueCode        bool
	InvoiceExpiry        time.Duration
	DataFile             string
}

// Load loads the configuration from environment variables and optionally from .env file.
func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		APIKey:               getEnv("API_KEY", "secret-api-key"),
		WebhookSecret:        getEnv("WEBHOOK_SECRET", "webhook-secret-token"),
		ClientWebhookURL:     getEnv("CLIENT_WEBHOOK_URL", ""),
		QRISStatic:           getEnv("QRIS_STATIC", ""),
		MerchantName:         getEnv("MERCHANT_NAME", "GoPay/QRIS Merchant"),
		UseUniqueCode:        getEnvBool("USE_UNIQUE_CODE", true),
		InvoiceExpiry:        time.Duration(getEnvInt("INVOICE_EXPIRY_MINUTES", 15)) * time.Minute,
		DataFile:             getEnv("DATA_FILE", "./data/invoices.json"),
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return strings.TrimSpace(val)
}

func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return defaultVal
	}
	return b
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return defaultVal
	}
	return n
}

// loadDotEnv reads key=value lines from a .env file and sets them if not already present in environment.
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return // File does not exist, ignore
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			// Strip quotes if present
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}
