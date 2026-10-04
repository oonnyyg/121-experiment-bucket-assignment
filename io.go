package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type User struct {
	ID    string
	Attrs map[string]string
}

// ReadUsers reads a CSV whose first column is user_id; remaining columns are
// attributes. User IDs must be unique and free of NUL bytes.
func ReadUsers(path string) ([]User, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open users csv: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read users header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	if len(header) == 0 || header[0] != "user_id" {
		return nil, fmt.Errorf("users csv: first column must be user_id")
	}
	seen := map[string]bool{}
	var users []User
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read users csv: %w", err)
		}
		id := rec[0]
		if id == "" {
			return nil, fmt.Errorf("users csv: empty user_id")
		}
		if strings.ContainsRune(id, 0) {
			return nil, fmt.Errorf("users csv: user_id %q contains NUL byte", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("users csv: duplicate user_id %q", id)
		}
		seen[id] = true
		attrs := map[string]string{}
		for i := 1; i < len(header) && i < len(rec); i++ {
			attrs[header[i]] = rec[i]
		}
		users = append(users, User{ID: id, Attrs: attrs})
	}
	return users, nil
}

type Record struct {
	UserID    string     `json:"user_id"`
	Decisions []Decision `json:"decisions"`
}

type summaryKey struct{ group, experiment, variant, reason string }

// Run assigns all users and streams JSONL decisions, then writes the summary
// CSV. Counts are derived from the same decisions, guaranteeing consistency.
func Run(cfg *Config, users []User, decisionsPath, summaryPath string) error {
	df, err := os.Create(decisionsPath)
	if err != nil {
		return fmt.Errorf("create decisions file: %w", err)
	}
	defer df.Close()
	enc := json.NewEncoder(df)

	counts := map[summaryKey]int{}
	var order []summaryKey
	add := func(k summaryKey) {
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}

	for _, u := range users {
		rec := Record{UserID: u.ID}
		for _, g := range cfg.Groups {
			d := decideGroup(g, u.ID, u.Attrs)
			rec.Decisions = append(rec.Decisions, d)
			add(summaryKey{g.Name, d.Experiment, d.Variant, d.Reason})
		}
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("write decisions: %w", err)
		}
	}
	if err := df.Close(); err != nil {
		return err
	}

	sf, err := os.Create(summaryPath)
	if err != nil {
		return fmt.Errorf("create summary file: %w", err)
	}
	defer sf.Close()
	w := csv.NewWriter(sf)
	if err := w.Write([]string{"group", "experiment", "variant", "reason", "count"}); err != nil {
		return err
	}
	for _, k := range order {
		if err := w.Write([]string{k.group, k.experiment, k.variant, k.reason,
			fmt.Sprintf("%d", counts[k])}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
