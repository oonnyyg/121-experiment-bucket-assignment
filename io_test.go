package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func writeUsersCSV(t *testing.T, path string, ids []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"user_id", "country"})
	for i, id := range ids {
		country := "US"
		if i%3 == 0 {
			country = "FR"
		}
		w.Write([]string{id, country})
	}
	w.Flush()
}

func TestEndToEndSummaryMatchesDetail(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"groups":[
	 {"name":"g1","salt":"s1","experiments":[
	   {"name":"e1","salt":"v1","start":0,"end":5000,
	    "variants":[{"name":"a","weight":5000},{"name":"b","weight":5000}]},
	   {"name":"e2","salt":"v2","start":5000,"end":8000,
	    "attribute":{"key":"country","values":["US"]},
	    "variants":[{"name":"on","weight":10000}]}]},
	 {"name":"g2","salt":"s2","experiments":[
	   {"name":"e3","salt":"v3","start":0,"end":10000,
	    "variants":[{"name":"x","weight":2500},{"name":"y","weight":7500}]}]}]}`), 0o644)
	ids := make([]string, 0, 5000)
	for i := 0; i < 5000; i++ {
		ids = append(ids, "user-"+strconv.Itoa(i))
	}
	usersPath := filepath.Join(dir, "users.csv")
	writeUsersCSV(t, usersPath, ids)
	decPath := filepath.Join(dir, "dec.jsonl")
	sumPath := filepath.Join(dir, "sum.csv")

	if err := run(cfgPath, usersPath, decPath, sumPath); err != nil {
		t.Fatal(err)
	}

	// Tally detail records.
	type key struct{ g, e, v, r string }
	tally := map[key]int{}
	f, _ := os.Open(decPath)
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		var rec Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatal(err)
		}
		n++
		if len(rec.Decisions) != 2 {
			t.Fatalf("expected 2 decisions per user, got %d", len(rec.Decisions))
		}
		for _, d := range rec.Decisions {
			tally[key{d.Group, d.Experiment, d.Variant, d.Reason}]++
			if d.Reason == "" && d.VariantBucket == nil {
				t.Fatal("assigned decision missing variant bucket")
			}
		}
	}
	if n != 5000 {
		t.Fatalf("expected 5000 records, got %d", n)
	}

	// Compare with summary CSV.
	sf, _ := os.Open(sumPath)
	defer sf.Close()
	recs, err := csv.NewReader(sf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if recs[0][0] != "group" {
		t.Fatal("bad summary header")
	}
	total := 0
	for _, row := range recs[1:] {
		c, _ := strconv.Atoi(row[4])
		total += c
		got := tally[key{row[0], row[1], row[2], row[3]}]
		if got != c {
			t.Errorf("summary %v count %d != detail %d", row[:4], c, got)
		}
	}
	if total != 10000 {
		t.Errorf("summary total %d != 2*5000", total)
	}
}

func TestUserOrderDoesNotAffectBuckets(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"groups":[{"name":"g","salt":"s","experiments":[
	 {"name":"e","salt":"v","start":0,"end":10000,
	  "variants":[{"name":"a","weight":5000},{"name":"b","weight":5000}]}]}]}`), 0o644)
	usersPath := filepath.Join(dir, "users.csv")
	writeUsersCSV(t, usersPath, []string{"u1", "u2", "u3"})
	p1 := filepath.Join(dir, "a.jsonl")
	if err := run(cfgPath, usersPath, p1, filepath.Join(dir, "a.csv")); err != nil {
		t.Fatal(err)
	}
	writeUsersCSV(t, usersPath, []string{"u3", "u1", "u2"})
	p2 := filepath.Join(dir, "b.jsonl")
	if err := run(cfgPath, usersPath, p2, filepath.Join(dir, "b.csv")); err != nil {
		t.Fatal(err)
	}
	load := func(p string) map[string]Record {
		f, _ := os.Open(p)
		defer f.Close()
		out := map[string]Record{}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var r Record
			json.Unmarshal(sc.Bytes(), &r)
			out[r.UserID] = r
		}
		return out
	}
	a, b := load(p1), load(p2)
	for id, ra := range a {
		rb := b[id]
		if ra.Decisions[0].GroupBucket != rb.Decisions[0].GroupBucket ||
			ra.Decisions[0].Variant != rb.Decisions[0].Variant {
			t.Errorf("user %s decision changed with order", id)
		}
	}
}

func TestDuplicateUserRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "users.csv")
	os.WriteFile(p, []byte("user_id\nu1\nu1\n"), 0o644)
	if _, err := ReadUsers(p); err == nil {
		t.Fatal("expected duplicate user error")
	}
}
