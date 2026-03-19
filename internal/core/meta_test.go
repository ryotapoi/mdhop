package core

import (
	"testing"
)

func TestNormalizeSortValue_String(t *testing.T) {
	tests := []struct {
		input       string
		wantValue   string
		wantWarning bool
	}{
		{"hello", "hello", false},
		{"Hello World", "Hello World", false},
		{"", "", false},
		{"123", "123", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, warn := NormalizeSortValue(tt.input, MetaTypeInfo{Name: MetaTypeString})
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
			if (warn != "") != tt.wantWarning {
				t.Errorf("warning = %q, wantWarning = %v", warn, tt.wantWarning)
			}
		})
	}
}

func TestNormalizeSortValue_Date(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValue   string
		wantWarning bool
	}{
		{"ISO date", "2024-01-15", "2024-01-15", false},
		{"short month/day", "2024-1-5", "2024-01-05", false},
		{"datetime no tz", "2024-01-15T10:30:00", "2024-01-15T10:30:00", false},
		{"slash date", "2024/01/15", "2024-01-15", false},
		{"datetime UTC", "2024-01-15T10:30:00Z", "2024-01-15T10:30:00Z", false},
		{"datetime +09:00", "2024-01-15T10:30:00+09:00", "2024-01-15T01:30:00Z", false},
		{"invalid", "not-a-date", "not-a-date", true},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warn := NormalizeSortValue(tt.input, MetaTypeInfo{Name: MetaTypeDate})
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
			if (warn != "") != tt.wantWarning {
				t.Errorf("warning = %q, wantWarning = %v", warn, tt.wantWarning)
			}
		})
	}
}

func TestNormalizeSortValue_Number(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValue   string
		wantWarning bool
	}{
		{"integer", "42", "100000000000000000042.00000000", false},
		{"negative", "-5", "09999999999999999999499999999", false},
		{"decimal", "3.14", "100000000000000000003.14000000", false},
		{"zero", "0", "100000000000000000000.00000000", false},
		{"positive sign", "+5", "100000000000000000005.00000000", false},
		{"negative zero", "-0", "100000000000000000000.00000000", false},
		{"negative zero decimal", "-0.0", "100000000000000000000.00000000", false},
		{"invalid", "abc", "abc", true},
		{"scientific", "1e20", "1e20", true},
		{"21 digits", "123456789012345678901", "123456789012345678901", true},
		{"empty", "", "", false},
		{"leading dot", ".5", "100000000000000000000.50000000", false},
		{"trailing dot", "0.", "100000000000000000000.00000000", false},
		{"dot only", ".", ".", true},
		{"long decimal", "3.123456789012", "100000000000000000003.123456789012", false},
		{"20 digit int", "12345678901234567890", "112345678901234567890.00000000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warn := NormalizeSortValue(tt.input, MetaTypeInfo{Name: MetaTypeNumber})
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
			if (warn != "") != tt.wantWarning {
				t.Errorf("warning = %q, wantWarning = %v", warn, tt.wantWarning)
			}
		})
	}
}

func TestNormalizeSortValue_NumberOrder(t *testing.T) {
	inputs := []string{"-10", "-5", "0", "3", "3.14", "42"}
	var prev string
	for i, input := range inputs {
		got, warn := NormalizeSortValue(input, MetaTypeInfo{Name: MetaTypeNumber})
		if warn != "" {
			t.Fatalf("unexpected warning for %q: %s", input, warn)
		}
		if i > 0 && got <= prev {
			t.Errorf("order violation: normalized(%q)=%q should be > normalized(%q)=%q", input, got, inputs[i-1], prev)
		}
		prev = got
	}
}

func TestNormalizeSortValue_Semver(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValue   string
		wantWarning bool
	}{
		{"3 segments", "v1.2.3", "00001.00002.00003", false},
		{"no v prefix", "2.10.0", "00002.00010.00000", false},
		{"2 segments", "1.2", "00001.00002.00000", false},
		{"4 segments", "1.2.3.4", "00001.00002.00003.00004", false},
		{"invalid", "not-semver", "not-semver", true},
		{"prerelease", "1.2.3-beta", "1.2.3-beta", true},
		{"empty", "", "", false},
		{"V prefix", "V1.0.0", "00001.00000.00000", false},
		{"single segment", "1", "1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warn := NormalizeSortValue(tt.input, MetaTypeInfo{Name: MetaTypeSemver})
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
			if (warn != "") != tt.wantWarning {
				t.Errorf("warning = %q, wantWarning = %v", warn, tt.wantWarning)
			}
		})
	}
}

func TestNormalizeSortValue_SemverOrder(t *testing.T) {
	inputs := []string{"v1.2", "v1.2.3", "v1.10.0", "v2.0.0"}
	var prev string
	for i, input := range inputs {
		got, warn := NormalizeSortValue(input, MetaTypeInfo{Name: MetaTypeSemver})
		if warn != "" {
			t.Fatalf("unexpected warning for %q: %s", input, warn)
		}
		if i > 0 && got <= prev {
			t.Errorf("order violation: normalized(%q)=%q should be > normalized(%q)=%q", input, got, inputs[i-1], prev)
		}
		prev = got
	}
}

func TestNormalizeSortValue_Ordered(t *testing.T) {
	info := MetaTypeInfo{
		Name:          MetaTypeOrdered,
		OrderedValues: []string{"low", "middle", "high", "critical"},
	}
	tests := []struct {
		name        string
		input       string
		wantValue   string
		wantWarning bool
	}{
		{"first", "low", "00000", false},
		{"last", "critical", "00003", false},
		{"middle", "high", "00002", false},
		{"unknown", "unknown", "unknown", true},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warn := NormalizeSortValue(tt.input, info)
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
			if (warn != "") != tt.wantWarning {
				t.Errorf("warning = %q, wantWarning = %v", warn, tt.wantWarning)
			}
		})
	}
}
