package qris

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateDynamicQRIS converts a Static QRIS into a Dynamic QRIS with a specific amount.
// - Sets Tag 01 (Point of Initiation Method) = "12" (Dynamic)
// - Injects/Updates Tag 54 (Transaction Amount)
// - Computes and appends Tag 6304 + CRC16 Checksum
func GenerateDynamicQRIS(staticQRIS string, amount int64) (string, error) {
	if strings.TrimSpace(staticQRIS) == "" {
		return "", errors.New("static QRIS template cannot be empty")
	}
	if amount <= 0 {
		return "", errors.New("amount must be greater than zero")
	}

	tags, err := ParseTLV(staticQRIS)
	if err != nil {
		return "", fmt.Errorf("failed to parse static QRIS: %w", err)
	}

	// 1. Update Tag 01 to "12" (Dynamic QR)
	tag01Found := false
	for i := range tags {
		if tags[i].Tag == "01" {
			tags[i].Value = "12"
			tags[i].Len = 2
			tag01Found = true
			break
		}
	}
	if !tag01Found {
		// Insert Tag 01 after Tag 00 if possible, or at index 1
		newTag01 := TLVTag{Tag: "01", Len: 2, Value: "12"}
		if len(tags) > 1 && tags[0].Tag == "00" {
			tags = append(tags[:1], append([]TLVTag{newTag01}, tags[1:]...)...)
		} else {
			tags = append([]TLVTag{newTag01}, tags...)
		}
	}

	// 2. Set or Update Tag 54 (Amount)
	amountStr := strconv.FormatInt(amount, 10)
	tag54Found := false
	for i := range tags {
		if tags[i].Tag == "54" {
			tags[i].Value = amountStr
			tags[i].Len = len(amountStr)
			tag54Found = true
			break
		}
	}

	if !tag54Found {
		newTag54 := TLVTag{Tag: "54", Len: len(amountStr), Value: amountStr}
		// Preferred insertion point: right before Tag 58 (Country Code) or Tag 53 (Currency)
		inserted := false
		for i, t := range tags {
			if t.Tag == "58" || t.Tag == "53" {
				tags = append(tags[:i], append([]TLVTag{newTag54}, tags[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted {
			tags = append(tags, newTag54)
		}
	}

	// 3. Rebuild payload
	reconstructed := BuildTLV(tags)

	// 4. Append Tag 6304 and compute CRC16
	checksumPayload := reconstructed + "6304"
	crc := CalculateCRC16(checksumPayload)

	return checksumPayload + crc, nil
}

// GenerateQRCodePNG generates PNG image bytes of the QR code string.
func GenerateQRCodePNG(content string, size int) ([]byte, error) {
	if size <= 0 {
		size = 320
	}
	return qrcode.Encode(content, qrcode.Medium, size)
}

// GenerateQRCodeBase64 generates a base64 Data URL for the QR code.
func GenerateQRCodeBase64(content string, size int) (string, error) {
	pngBytes, err := GenerateQRCodePNG(content, size)
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	return "data:image/png;base64," + b64, nil
}
