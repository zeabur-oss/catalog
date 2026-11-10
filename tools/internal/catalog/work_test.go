package catalog

import "testing"

func TestQuantitiesAboveZero(t *testing.T) {
	for _, c := range []struct {
		name string
		q    Quantities
		ok   bool
	}{
		{"binary and milli", Quantities{"memory": "256Mi", "cpu": "100m"}, true},
		{"numbers", Quantities{"memory": 268435457, "cpu": 1}, true},
		{"none", nil, true},
		{"unknown resource", Quantities{"gpu": "1"}, false},
		{"not a quantity", Quantities{"memory": "lots"}, false},
		{"zero", Quantities{"memory": "0"}, false},
		{"negative", Quantities{"cpu": "-1"}, false},
	} {
		if err := c.q.check(); (err == nil) != c.ok {
			t.Errorf("%s: %v, want ok %v", c.name, err, c.ok)
		}
	}
}
