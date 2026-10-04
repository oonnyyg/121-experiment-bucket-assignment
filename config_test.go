package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfigJSON() string {
	return `{"groups":[{"name":"g1","salt":"s","experiments":[
		{"name":"e1","salt":"v","start":0,"end":5000,
		 "variants":[{"name":"a","weight":5000},{"name":"b","weight":5000}]}]}]}`
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigValid(t *testing.T) {
	if _, err := LoadConfig(writeTempConfig(t, validConfigJSON())); err != nil {
		t.Fatal(err)
	}
}

func TestConfigErrors(t *testing.T) {
	cases := map[string]string{
		"empty groups":               `{"groups":[]}`,
		"dup group":                  `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e","salt":"v","start":0,"end":1,"variants":[{"name":"a","weight":10000}]}]},{"name":"g","salt":"s","experiments":[{"name":"f","salt":"v","start":0,"end":1,"variants":[{"name":"a","weight":10000}]}]}]}`,
		"dup experiment":             `{"groups":[{"name":"g1","salt":"s","experiments":[{"name":"e","salt":"v","start":0,"end":1,"variants":[{"name":"a","weight":10000}]}]},{"name":"g2","salt":"s","experiments":[{"name":"e","salt":"v","start":0,"end":1,"variants":[{"name":"a","weight":10000}]}]}]}`,
		"overlap":                    `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e1","salt":"v","start":0,"end":6000,"variants":[{"name":"a","weight":10000}]},{"name":"e2","salt":"v","start":5000,"end":9000,"variants":[{"name":"a","weight":10000}]}]}]}`,
		"adjacent ok then bad range": `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e1","salt":"v","start":0,"end":5000,"variants":[{"name":"a","weight":10000}]},{"name":"e2","salt":"v","start":5000,"end":10001,"variants":[{"name":"a","weight":10000}]}]}]}`,
		"negative start":             `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e","salt":"v","start":-1,"end":5,"variants":[{"name":"a","weight":10000}]}]}]}`,
		"empty range":                `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e","salt":"v","start":5,"end":5,"variants":[{"name":"a","weight":10000}]}]}]}`,
		"weights not 10000":          `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e","salt":"v","start":0,"end":5,"variants":[{"name":"a","weight":9999}]}]}]}`,
		"zero weight":                `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e","salt":"v","start":0,"end":5,"variants":[{"name":"a","weight":0},{"name":"b","weight":10000}]}]}]}`,
		"nul in salt":                `{"groups":[{"name":"g","salt":"s","experiments":[{"name":"e","salt":"v","start":0,"end":5,"variants":[{"name":"a","weight":10000}]}]}]}`,
	}
	// Inject NUL into the "nul in salt" case (JSON string with literal NUL).
	cases["nul in salt"] = strings.Replace(cases["nul in salt"], `"salt":"s"`, "\"salt\":\"s\x00\"", 1)
	for name, body := range cases {
		if _, err := LoadConfig(writeTempConfig(t, body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestAdjacentRangesAllowed(t *testing.T) {
	body := `{"groups":[{"name":"g","salt":"s","experiments":[
		{"name":"e1","salt":"v","start":0,"end":5000,"variants":[{"name":"a","weight":10000}]},
		{"name":"e2","salt":"v","start":5000,"end":10000,"variants":[{"name":"a","weight":10000}]}]}]}`
	if _, err := LoadConfig(writeTempConfig(t, body)); err != nil {
		t.Fatal(err)
	}
}

func TestConfigErrorProducesNoOutput(t *testing.T) {
	dir := t.TempDir()
	bad := writeTempConfig(t, `{"groups":[]}`)
	users := filepath.Join(dir, "users.csv")
	os.WriteFile(users, []byte("user_id\nu1\n"), 0o644)
	dec := filepath.Join(dir, "dec.jsonl")
	sum := filepath.Join(dir, "sum.csv")
	if err := run(bad, users, dec, sum); err == nil {
		t.Fatal("expected error")
	}
	for _, p := range []string{dec, sum} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("partial output %s should not exist", p)
		}
	}
}
