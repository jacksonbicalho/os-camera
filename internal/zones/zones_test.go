package zones_test

import (
	"testing"

	"camera/internal/zones"
)

func TestIsExclude(t *testing.T) {
	cases := []struct {
		zType    string
		expected bool
	}{
		{"", true},
		{"exclude", true},
		{"detect", false},
	}
	for _, c := range cases {
		z := zones.Zone{Type: c.zType}
		if got := z.IsExclude(); got != c.expected {
			t.Errorf("Zone{Type:%q}.IsExclude() = %v, want %v", c.zType, got, c.expected)
		}
	}
}
