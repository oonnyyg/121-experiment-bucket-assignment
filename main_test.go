package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestProcessEndToEnd(t *testing.T) {
	cfg := testConfig()
	cfg.Groups = append(cfg.Groups, Group{
		Name: "g2", Salt: "s2",
		Experiments: []Experiment{{
			Name: "e3", Salt: "es3", BucketFrom: 0, BucketTo: 10000,
			Variants: []Variant{{Name: "on", Weight: 5000}, {Name: "off", Weight: 5000}},
		}},
	})
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	var users []user
	for i := 0; i < 500; i++ {
		users = append(users, user{id: "u" + itoa(i), attrs: map[string]string{"country": "US"}})
	}
	var decBuf, sumBuf bytes.Buffer
	total, err := process(cfg, users, &decBuf, &sumBuf)
	if err != nil {
		t.Fatal(err)
	}
	if total != len(users)*len(cfg.Groups) {
		t.Fatalf("total = %d, want %d", total, len(users)*len(cfg.Groups))
	}
	lines := strings.Split(strings.TrimSpace(decBuf.String()), "\n")
	if len(lines) != total {
		t.Fatalf("jsonl lines = %d, want %d", len(lines), total)
	}
	perUser := map[string]map[string]int{}
	for _, ln := range lines {
		var d Decision
		if err := json.Unmarshal([]byte(ln), &d); err != nil {
			t.Fatal(err)
		}
		if perUser[d.UserID] == nil {
			perUser[d.UserID] = map[string]int{}
		}
		perUser[d.UserID][d.Group]++
	}
	for uid, groups := range perUser {
		if len(groups) != len(cfg.Groups) {
			t.Fatalf("user %s has %d group decisions", uid, len(groups))
		}
		for g, n := range groups {
			if n != 1 {
				t.Fatalf("user %s group %s: %d decisions", uid, g, n)
			}
		}
	}
	// summary counts must equal detail count
	r := csv.NewReader(&sumBuf)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, row := range rows[1:] {
		n, err := strconv.Atoi(row[4])
		if err != nil {
			t.Fatal(err)
		}
		sum += n
	}
	if sum != total {
		t.Fatalf("summary sum %d != decisions %d", sum, total)
	}
}

func TestReadUsersDuplicate(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/users.csv"
	if err := writeFile(p, "user_id,country\nu1,US\nu1,FR\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := readUsers(p); err == nil {
		t.Fatal("expected duplicate user error")
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
