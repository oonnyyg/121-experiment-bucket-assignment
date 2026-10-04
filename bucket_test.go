package main

import "testing"

func TestBucketOfKnownVector(t *testing.T) {
	got, err := bucketOf("group", "salt", "g1", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got != 4 {
		t.Fatalf("bucket = %d, want 4", got)
	}
}

func TestBucketOfStableAcrossOrder(t *testing.T) {
	a, _ := bucketOf("variant", "s2", "exp", "u1")
	b, _ := bucketOf("variant", "s2", "exp", "u1")
	if a != b {
		t.Fatalf("not deterministic: %d vs %d", a, b)
	}
	if a < 0 || a >= totalBuckets {
		t.Fatalf("bucket %d out of range", a)
	}
}

func TestBucketOfRejectsNUL(t *testing.T) {
	if _, err := bucketOf("group", "sa\x00lt", "g", "u"); err == nil {
		t.Fatal("expected error for NUL in salt")
	}
}

func testConfig() *Config {
	return &Config{Groups: []Group{
		{
			Name: "g1",
			Salt: "gs",
			Experiments: []Experiment{
				{
					Name: "e1", Salt: "es",
					BucketFrom: 0, BucketTo: 5000,
					Variants: []Variant{{Name: "a", Weight: 5000}, {Name: "b", Weight: 5000}},
				},
				{
					Name: "e2", Salt: "es2",
					BucketFrom: 5000, BucketTo: 6000,
					Attribute: "country", Allowed: []string{"US"},
					Variants: []Variant{{Name: "x", Weight: 10000}},
				},
			},
		},
	}}
}

func TestAssignGroupNoExperiment(t *testing.T) {
	cfg := testConfig()
	// find a user whose group bucket falls in [6000,10000)
	for i := 0; i < 100000; i++ {
		uid := "user" + string(rune('a'+i%26)) + itoa(i)
		gb, _ := bucketOf("group", "gs", "g1", uid)
		if gb >= 6000 {
			d, err := assignGroup(&cfg.Groups[0], uid, nil)
			if err != nil {
				t.Fatal(err)
			}
			if d.Reason != reasonNoExperiment || d.Experiment != "" {
				t.Fatalf("got %+v", d)
			}
			return
		}
	}
	t.Fatal("no user found in uncovered range")
}

func TestAssignGroupAttributeGate(t *testing.T) {
	cfg := testConfig()
	var uid string
	for i := 0; i < 100000; i++ {
		cand := "gate" + itoa(i)
		gb, _ := bucketOf("group", "gs", "g1", cand)
		if gb >= 5000 && gb < 6000 {
			uid = cand
			break
		}
	}
	if uid == "" {
		t.Fatal("no user found in e2 range")
	}
	// missing attribute
	d, _ := assignGroup(&cfg.Groups[0], uid, map[string]string{})
	if d.Reason != reasonAttributeMissing || d.Experiment != "" {
		t.Fatalf("missing attr: %+v", d)
	}
	// disallowed value: must not fall through to e1
	d, _ = assignGroup(&cfg.Groups[0], uid, map[string]string{"country": "FR"})
	if d.Reason != reasonAttributeDenied || d.Experiment != "" {
		t.Fatalf("denied attr: %+v", d)
	}
	// allowed value
	d, _ = assignGroup(&cfg.Groups[0], uid, map[string]string{"country": "US"})
	if d.Experiment != "e2" || d.Variant != "x" || d.Reason != "" {
		t.Fatalf("allowed: %+v", d)
	}
}

func TestVariantSplitMatchesWeights(t *testing.T) {
	e := &Experiment{
		Name: "e", Salt: "s",
		Variants: []Variant{{Name: "a", Weight: 2500}, {Name: "b", Weight: 7500}},
	}
	counts := map[string]int{}
	n := 20000
	for i := 0; i < n; i++ {
		uid := "u" + itoa(i)
		vb, _ := bucketOf("variant", e.Salt, e.Name, uid)
		acc := 0
		for _, v := range e.Variants {
			acc += v.Weight
			if vb < acc {
				counts[v.Name]++
				break
			}
		}
	}
	ratio := float64(counts["a"]) / float64(n)
	if ratio < 0.22 || ratio > 0.28 {
		t.Fatalf("variant a ratio %f far from 0.25", ratio)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [8]byte
	p := len(buf)
	for i > 0 {
		p--
		buf[p] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[p:])
}
