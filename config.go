package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const totalBuckets = 10000

type AttributeRule struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

type Variant struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type Experiment struct {
	Name      string         `json:"name"`
	Salt      string         `json:"salt"`
	Start     int            `json:"start"`
	End       int            `json:"end"`
	Attribute *AttributeRule `json:"attribute,omitempty"`
	Variants  []Variant      `json:"variants"`
}

type Group struct {
	Name        string       `json:"name"`
	Salt        string       `json:"salt"`
	Experiments []Experiment `json:"experiments"`
}

type Config struct {
	Groups []Group `json:"groups"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func noNUL(kind, s string) error {
	if strings.ContainsRune(s, 0) {
		return fmt.Errorf("%s must not contain NUL byte", kind)
	}
	return nil
}

func (c *Config) Validate() error {
	if len(c.Groups) == 0 {
		return fmt.Errorf("config must declare at least one group")
	}
	groupNames := map[string]bool{}
	expNames := map[string]bool{}
	for gi, g := range c.Groups {
		if g.Name == "" {
			return fmt.Errorf("groups[%d]: name is required", gi)
		}
		if err := noNUL("group name", g.Name); err != nil {
			return err
		}
		if err := noNUL("group salt", g.Salt); err != nil {
			return err
		}
		if groupNames[g.Name] {
			return fmt.Errorf("duplicate group name %q", g.Name)
		}
		groupNames[g.Name] = true
		if len(g.Experiments) == 0 {
			return fmt.Errorf("group %q: must declare at least one experiment", g.Name)
		}
		type seg struct{ start, end int }
		segs := []seg{}
		for ei, e := range g.Experiments {
			if e.Name == "" {
				return fmt.Errorf("group %q experiments[%d]: name is required", g.Name, ei)
			}
			if err := noNUL("experiment name", e.Name); err != nil {
				return err
			}
			if err := noNUL("experiment salt", e.Salt); err != nil {
				return err
			}
			if expNames[e.Name] {
				return fmt.Errorf("duplicate experiment name %q", e.Name)
			}
			expNames[e.Name] = true
			if e.Start < 0 || e.End > totalBuckets || e.Start >= e.End {
				return fmt.Errorf("experiment %q: invalid bucket range [%d,%d)", e.Name, e.Start, e.End)
			}
			segs = append(segs, seg{e.Start, e.End})
			if e.Attribute != nil {
				if e.Attribute.Key == "" {
					return fmt.Errorf("experiment %q: attribute key is required", e.Name)
				}
				if len(e.Attribute.Values) == 0 {
					return fmt.Errorf("experiment %q: attribute values must be non-empty", e.Name)
				}
			}
			if len(e.Variants) == 0 {
				return fmt.Errorf("experiment %q: must declare at least one variant", e.Name)
			}
			sum := 0
			seenVar := map[string]bool{}
			for vi, v := range e.Variants {
				if v.Name == "" {
					return fmt.Errorf("experiment %q variants[%d]: name is required", e.Name, vi)
				}
				if seenVar[v.Name] {
					return fmt.Errorf("experiment %q: duplicate variant name %q", e.Name, v.Name)
				}
				seenVar[v.Name] = true
				if v.Weight <= 0 {
					return fmt.Errorf("experiment %q variant %q: weight must be a positive integer", e.Name, v.Name)
				}
				sum += v.Weight
			}
			if sum != totalBuckets {
				return fmt.Errorf("experiment %q: variant weights sum to %d, want %d", e.Name, sum, totalBuckets)
			}
		}
		sort.Slice(segs, func(i, j int) bool { return segs[i].start < segs[j].start })
		for i := 1; i < len(segs); i++ {
			if segs[i].start < segs[i-1].end {
				return fmt.Errorf("group %q: overlapping experiment bucket ranges", g.Name)
			}
		}
	}
	return nil
}
