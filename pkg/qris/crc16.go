package qris

import "fmt"

// CalculateCRC16 computes the standard EMVCo CRC16-CCITT checksum for a QRIS payload.
// Polynomial: 0x1021, Initial value: 0xFFFF.
func CalculateCRC16(payload string) string {
	crc := uint16(0xFFFF)
	data := []byte(payload)

	for _, b := range data {
		crc ^= uint16(b) << 8
		for j := 0; j < 8; j++ {
			if (crc & 0x8000) != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc = crc << 1
			}
		}
	}

	return fmt.Sprintf("%04X", crc)
}
