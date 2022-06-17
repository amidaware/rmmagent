package utils

import (
	"testing"
)

func TestByteCountSI(t *testing.T) {
	var bytes uint64 = 1048576
	mb := ByteCountSI(bytes)
	if mb != "1.0 MB" {
		t.Errorf("Expected 1.0 MB, got %s", mb)
	}
}

func TestRemoveWinNewLines(t *testing.T) {
	result := RemoveWinNewLines("test\r\n")
	if result != "test\n" {
		t.Fatalf("Expected testing\\n, got %s", result)
	}

	t.Logf("Result: %s", result)
}

func TestStripAll(t *testing.T) {
	result := StripAll("   test\r\n    ")
	if result != "test" {
		t.Fatalf("Expecte test, got %s", result)
	}

	t.Log("Test result expected")
}
