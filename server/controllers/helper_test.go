package controllers

import "testing"

func TestNumberToJidAcceptsInternationalNumbers(t *testing.T) {
	validNumbers := []string{
		"5561999211277", // Brazil (13 digits)
		"13233923870",   // United States, +1 (11 digits)
		"34662418782",   // Spain, +34 (11 digits)
	}

	for _, number := range validNumbers {
		if _, err := numberToJid(number); err != nil {
			t.Errorf("numberToJid(%q) returned unexpected error: %v", number, err)
		}
	}
}

func TestNumberToJidRejectsTooShortNumbers(t *testing.T) {
	tooShort := []string{"123", "5561"}

	for _, number := range tooShort {
		if _, err := numberToJid(number); err == nil {
			t.Errorf("numberToJid(%q) expected an error, got nil", number)
		}
	}
}
