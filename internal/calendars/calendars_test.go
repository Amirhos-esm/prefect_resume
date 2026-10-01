package calendars

import "testing"

func TestFormatShamsi(t *testing.T) {
	tests := map[string]string{
		"2024-03-20": "1403/01/01",
		"2024/03/20": "1403/01/01",
		"2024-01-01": "1402/10/11",
		"2024-03":    "1402/12",
		"2024/03":    "1402/12",
		"2014":       "1393/01",
		"Present":    "Present",
		"":           "",
	}
	for input, want := range tests {
		if got := Format(input, Shamsi); got != want {
			t.Errorf("Format(%q, Shamsi) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatGregorianPreservesInput(t *testing.T) {
	if got := Format("2024-03-20", Gregorian); got != "2024-03-20" {
		t.Fatalf("Gregorian date changed to %q", got)
	}
}
