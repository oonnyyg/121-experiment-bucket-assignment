package main

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func refBucket(label, salt, object, userID string) int {
	var buf []byte
	for _, p := range []string{label, salt, object, userID} {
		buf = append(buf, []byte(p)...)
		buf = append(buf, 0)
	}
	sum := sha256.Sum256(buf)
	return int(binary.BigEndian.Uint64(sum[:8]) % 10000)
}

func TestBucketMatchesSpec(t *testing.T) {
	for _, u := range []string{"u1", "user-42", "张三", ""} {
		got := bucket("group", "s1", "g1", u)
		want := refBucket("group", "s1", "g1", u)
		if got != want {
			t.Fatalf("bucket mismatch for %q: got %d want %d", u, got, want)
		}
		if got < 0 || got >= 10000 {
			t.Fatalf("bucket out of range: %d", got)
		}
	}
}

func TestBucketStableAcrossOrder(t *testing.T) {
	a := bucket("variant", "salt", "exp", "u")
	b := bucket("variant", "salt", "exp", "u")
	if a != b {
		t.Fatal("bucket not deterministic")
	}
	if bucket("group", "salt", "exp", "u") == bucket("variant", "salt", "exp", "u") &&
		bucket("group", "salt2", "exp", "u") == bucket("group", "salt", "exp", "u") {
		t.Fatal("labels/salts should change inputs")
	}
}

func testConfig() *Config {
	return &Config{Groups: []Group{
		{
			Name: "g1", Salt: "gs1",
			Experiments: []Experiment{
				{Name: "e1", Salt: "es1", Start: 0, End: 5000,
					Variants: []Variant{{Name: "a", Weight: 5000}, {Name: "b", Weight: 5000}}},
				{Name: "e2", Salt: "es2", Start: 5000, End: 8000,
					Attribute: &AttributeRule{Key: "country", Values: []string{"US", "CA"}},
					Variants:  []Variant{{Name: "on", Weight: 10000}}},
			},
		},
		{
			Name: "g2", Salt: "gs2",
			Experiments: []Experiment{
				{Name: "e3", Salt: "es3", Start: 0, End: 10000,
					Variants: []Variant{{Name: "x", Weight: 1}, {Name: "y", Weight: 9999}}},
			},
		},
	}}
}

func TestDecideGroupBoundaries(t *testing.T) {
	cfg := testConfig()
	g := cfg.Groups[0]
	// Find users landing on boundary buckets 0, 4999, 5000, 7999, 8000.
	seen := map[int]string{}
	for i := 0; i < 2000000 && len(seen) < 5; i++ {
		u := "u" + string(rune(i))
		b := bucket("group", "gs1", "g1", u)
		for _, want := range []int{0, 4999, 5000, 7999, 8000} {
			if b == want {
				if _, ok := seen[want]; !ok {
					seen[want] = u
				}
			}
		}
	}
	if len(seen) != 5 {
		t.Fatalf("could not find users for all boundary buckets: %v", seen)
	}
	attrs := map[string]string{"country": "US"}
	cases := []struct {
		bucket int
		exp    string
		reason string
	}{
		{0, "e1", ""},
		{4999, "e1", ""},
		{5000, "e2", ""},
		{7999, "e2", ""},
		{8000, "", ReasonBucketNotCovered},
	}
	for _, c := range cases {
		d := decideGroup(g, seen[c.bucket], attrs)
		if d.GroupBucket != c.bucket || d.Experiment != c.exp || d.Reason != c.reason {
			t.Errorf("bucket %d: got %+v", c.bucket, d)
		}
		if c.exp == "" && d.VariantBucket != nil {
			t.Errorf("bucket %d: variant bucket should be nil when unassigned", c.bucket)
		}
	}
}

func TestAttributeGate(t *testing.T) {
	cfg := testConfig()
	g := cfg.Groups[0]
	// Find a user in e2's range.
	var uid string
	for i := 0; ; i++ {
		u := "u" + string(rune(i))
		b := bucket("group", "gs1", "g1", u)
		if b >= 5000 && b < 8000 {
			uid = u
			break
		}
	}
	d := decideGroup(g, uid, map[string]string{"country": "CA"})
	if d.Experiment != "e2" || d.Variant != "on" {
		t.Fatalf("expected e2/on, got %+v", d)
	}
	// Missing attribute -> unassigned, no fallthrough to other experiments.
	d = decideGroup(g, uid, map[string]string{})
	if d.Reason != ReasonAttributeMismatch || d.Experiment != "" {
		t.Fatalf("missing attr: got %+v", d)
	}
	// Non-matching attribute.
	d = decideGroup(g, uid, map[string]string{"country": "FR"})
	if d.Reason != ReasonAttributeMismatch {
		t.Fatalf("wrong attr: got %+v", d)
	}
}

func TestVariantWeights(t *testing.T) {
	cfg := testConfig()
	g := cfg.Groups[1]
	counts := map[string]int{}
	for i := 0; i < 20000; i++ {
		u := "u" + string(rune(i+100000))
		d := decideGroup(g, u, nil)
		if d.Experiment != "e3" || d.Reason != "" {
			t.Fatalf("full-coverage experiment must assign everyone: %+v", d)
		}
		counts[d.Variant]++
	}
	if counts["x"] == 0 || counts["y"] == 0 {
		t.Fatalf("both variants should appear: %v", counts)
	}
	if counts["x"]+counts["y"] != 20000 {
		t.Fatalf("counts inconsistent: %v", counts)
	}
	// Weight 1/10000: expect roughly 2 for x out of 20000; allow slack.
	if counts["x"] > 20 {
		t.Fatalf("variant x over-represented: %v", counts)
	}
}

func TestGroupsIndependent(t *testing.T) {
	cfg := testConfig()
	d1 := decideGroup(cfg.Groups[0], "u1", nil)
	d2 := decideGroup(cfg.Groups[1], "u1", nil)
	if d1.Group == d2.Group {
		t.Fatal("same group")
	}
	// e3 covers all buckets, so u1 is always assigned in g2 regardless of g1.
	if d2.Experiment != "e3" {
		t.Fatalf("g2 should assign independently: %+v", d2)
	}
}
