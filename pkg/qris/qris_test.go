package qris

import (
	"strings"
	"testing"
)

func TestCalculateCRC16(t *testing.T) {
	// Standard test: CRC16 of "123456789" with CCITT (0xFFFF, 0x1021) is 0x29B1
	input := "123456789"
	expected := "29B1"
	result := CalculateCRC16(input)
	if result != expected {
		t.Fatalf("expected CRC %s, got %s", expected, result)
	}
}

func TestGenerateDynamicQRIS(t *testing.T) {
	// Sample dummy static QRIS
	staticQR := "00020101021153033605802ID5912MerchantTest6007Jakarta6304ABCD"
	amount := int64(25000)

	dynamicQR, err := GenerateDynamicQRIS(staticQR, amount)
	if err != nil {
		t.Fatalf("GenerateDynamicQRIS failed: %v", err)
	}

	// Must contain Tag 01 with value 12
	if !strings.Contains(dynamicQR, "010212") {
		t.Errorf("expected dynamic QRIS to contain '010212', got %s", dynamicQR)
	}

	// Must contain Tag 54 with amount 25000: Tag "54" + Len "05" + "25000"
	if !strings.Contains(dynamicQR, "540525000") {
		t.Errorf("expected dynamic QRIS to contain '540525000', got %s", dynamicQR)
	}

	// Must end with 6304 + 4 hex chars
	idx63 := strings.LastIndex(dynamicQR, "6304")
	if idx63 == -1 || len(dynamicQR)-idx63 != 8 {
		t.Errorf("invalid CRC16 suffix in dynamic QRIS: %s", dynamicQR)
	}

	// Verify CRC matches
	dataPart := dynamicQR[:idx63+4]
	expectedCRC := CalculateCRC16(dataPart)
	actualCRC := dynamicQR[idx63+4:]
	if expectedCRC != actualCRC {
		t.Errorf("expected CRC %s, got %s", expectedCRC, actualCRC)
	}
}
