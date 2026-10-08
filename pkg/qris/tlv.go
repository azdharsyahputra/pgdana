package qris

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// TLVTag represents a single EMVCo Tag-Length-Value entry.
type TLVTag struct {
	Tag   string
	Len   int
	Value string
}

// ParseTLV parses an EMVCo string payload into a slice of TLVTag.
func ParseTLV(payload string) ([]TLVTag, error) {
	// Strip trailing CRC16 tag if present (Tag 63)
	idx63 := strings.LastIndex(payload, "6304")
	if idx63 != -1 && idx63+8 <= len(payload) {
		payload = payload[:idx63]
	}

	var tags []TLVTag
	i := 0
	for i < len(payload) {
		if i+4 > len(payload) {
			break
		}

		tag := payload[i : i+2]
		lenStr := payload[i+2 : i+4]
		length, err := strconv.Atoi(lenStr)
		if err != nil || length < 0 {
			return nil, fmt.Errorf("invalid tag length at index %d: %s", i+2, lenStr)
		}

		valEnd := i + 4 + length
		if valEnd > len(payload) {
			return nil, fmt.Errorf("tag %s value truncated (expected %d chars)", tag, length)
		}

		val := payload[i+4 : valEnd]
		tags = append(tags, TLVTag{
			Tag:   tag,
			Len:   length,
			Value: val,
		})

		i = valEnd
	}

	if len(tags) == 0 {
		return nil, errors.New("empty or invalid TLV payload")
	}

	return tags, nil
}

// BuildTLV reconstructs an EMVCo string from a slice of TLVTag without the CRC tag.
func BuildTLV(tags []TLVTag) string {
	var sb strings.Builder
	for _, t := range tags {
		valLen := len(t.Value)
		sb.WriteString(t.Tag)
		sb.WriteString(fmt.Sprintf("%02d", valLen))
		sb.WriteString(t.Value)
	}
	return sb.String()
}
