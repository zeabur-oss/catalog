package catalog

import (
	"reflect"
	"testing"
)

func TestQuantitiesRoundUp(t *testing.T) {
	cases := []struct {
		name string
		q    Quantities
		want *Amounts
	}{
		{"binary and milli", Quantities{"memory": "256Mi", "cpu": "100m"}, &Amounts{MemoryMiB: 256, CPUMillis: 100}},
		{"numbers", Quantities{"memory": 268435457, "cpu": 1}, &Amounts{MemoryMiB: 257, CPUMillis: 1000}},
		{"decimal and micro", Quantities{"memory": "300M", "cpu": "1500u"}, &Amounts{MemoryMiB: 287, CPUMillis: 2}},
		{"one resource", Quantities{"memory": "1Gi"}, &Amounts{MemoryMiB: 1024}},
		{"none", nil, nil},
	}
	for _, c := range cases {
		got, err := c.q.Amounts()
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestQuantitiesFail(t *testing.T) {
	for _, c := range []struct {
		name string
		q    Quantities
	}{
		{"unknown resource", Quantities{"gpu": "1"}},
		{"not a quantity", Quantities{"memory": "lots"}},
		{"zero", Quantities{"memory": "0"}},
		{"negative", Quantities{"cpu": "-1"}},
	} {
		if _, err := c.q.Amounts(); err == nil {
			t.Errorf("%s: want an error", c.name)
		}
	}
}

func TestIndexResources(t *testing.T) {
	c := &Component{ID: "builds/x", Resources: &Resources{Work: []Work{
		{
			Name:     "build",
			Requests: Quantities{"cpu": "100m", "memory": "256Mi"},
			Limits:   Quantities{"cpu": "1", "memory": "1Gi"},
			Use:      Quantities{"memory": "500Mi"},
		},
		{Name: "load", Use: Quantities{"memory": "64Mi"}},
	}}}
	got, err := c.IndexResources()
	if err != nil {
		t.Fatal(err)
	}
	want := &IndexResources{Work: []IndexWork{
		{
			Name:     "build",
			Requests: &Amounts{MemoryMiB: 256, CPUMillis: 100},
			Limits:   &Amounts{MemoryMiB: 1024, CPUMillis: 1000},
			Use:      &Amounts{MemoryMiB: 500},
		},
		{Name: "load", Use: &Amounts{MemoryMiB: 64}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	c.Resources.Work[1].Limits = Quantities{"memory": "a lot"}
	if _, err := c.IndexResources(); err == nil {
		t.Error("a bad limit was accepted")
	}
}
