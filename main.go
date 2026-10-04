package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
)

func run() error {
	configPath := flag.String("config", "config.json", "experiment config JSON")
	usersPath := flag.String("users", "users.csv", "input CSV: user_id plus attribute columns")
	decisionsPath := flag.String("decisions", "decisions.jsonl", "output JSONL decisions")
	summaryPath := flag.String("summary", "summary.csv", "output CSV summary")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	users, err := readUsers(*usersPath)
	if err != nil {
		return fmt.Errorf("users: %w", err)
	}

	decFile, err := os.Create(*decisionsPath)
	if err != nil {
		return err
	}
	sumFile, err := os.Create(*summaryPath)
	if err != nil {
		decFile.Close()
		return err
	}
	total, err := process(cfg, users, decFile, sumFile)
	decFile.Close()
	sumFile.Close()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d decisions for %d users x %d groups\n", total, len(users), len(cfg.Groups))
	return nil
}

// process writes one JSONL decision per user per group and a CSV summary,
// returning the number of decisions written.
func process(cfg *Config, users []user, decOut io.Writer, sumOut io.Writer) (int, error) {
	enc := json.NewEncoder(decOut)
	type key struct{ group, experiment, variant, reason string }
	counts := map[key]int{}
	total := 0
	for _, u := range users {
		for gi := range cfg.Groups {
			d, err := assignGroup(&cfg.Groups[gi], u.id, u.attrs)
			if err != nil {
				return 0, err
			}
			if err := enc.Encode(d); err != nil {
				return 0, err
			}
			counts[key{d.Group, d.Experiment, d.Variant, d.Reason}]++
			total++
		}
	}
	w := csv.NewWriter(sumOut)
	if err := w.Write([]string{"group", "experiment", "variant", "reason", "count"}); err != nil {
		return 0, err
	}
	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.group != b.group {
			return a.group < b.group
		}
		if a.experiment != b.experiment {
			return a.experiment < b.experiment
		}
		if a.variant != b.variant {
			return a.variant < b.variant
		}
		return a.reason < b.reason
	})
	sum := 0
	for _, k := range keys {
		sum += counts[k]
		if err := w.Write([]string{k.group, k.experiment, k.variant, k.reason, strconv.Itoa(counts[k])}); err != nil {
			return 0, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return 0, err
	}
	if sum != total {
		return 0, fmt.Errorf("summary count %d does not match decisions %d", sum, total)
	}
	return total, nil
}

type user struct {
	id    string
	attrs map[string]string
}

func readUsers(path string) ([]user, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if len(header) == 0 || header[0] != "user_id" {
		return nil, fmt.Errorf("first column must be user_id")
	}
	var users []user
	seen := map[string]bool{}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, attrs, err := splitAttrs(header, row)
		if err != nil {
			return nil, err
		}
		if id == "" {
			return nil, fmt.Errorf("empty user_id")
		}
		if err := noNUL(id); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate user_id %q", id)
		}
		seen[id] = true
		users = append(users, user{id: id, attrs: attrs})
	}
	return users, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
