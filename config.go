package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const totalBuckets = 10000

type Variant struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type Experiment struct {
	Name       string    `json:"name"`
	Salt       string    `json:"salt"`
	BucketFrom int       `json:"bucket_from"`
	BucketTo   int       `json:"bucket_to"`
	Attribute  string    `json:"attribute,omitempty"`
	Allowed    []string  `json:"allowed,omitempty"`
	Variants   []Variant `json:"variants"`
}

type Group struct {
	Name        string       `json:"name"`
	Salt        string       `json:"salt"`
	Experiments []Experiment `json:"experiments"`
}

type Config struct {
	Groups []Group `json:"groups"`
}

func noNUL(parts ...string) error {
	for _, p := range parts {
		if strings.ContainsRune(p, 0) {
			return fmt.Errorf("value %q contains NUL byte", p)
		}
	}
	return nil
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Groups) == 0 {
		return fmt.Errorf("config must declare at least one group")
	}
	groupNames := map[string]bool{}
	expNames := map[string]bool{}
	for gi := range c.Groups {
		g := &c.Groups[gi]
		if g.Name == "" {
			return fmt.Errorf("group %d: empty name", gi)
		}
		if groupNames[g.Name] {
			return fmt.Errorf("duplicate group name %q", g.Name)
		}
		groupNames[g.Name] = true
		if err := noNUL(g.Name, g.Salt); err != nil {
			return fmt.Errorf("group %q: %w", g.Name, err)
		}
		type seg struct{ from, to int }
		var segs []seg
		for ei := range g.Experiments {
			e := &g.Experiments[ei]
			if e.Name == "" {
				return fmt.Errorf("group %q: experiment %d: empty name", g.Name, ei)
			}
			if expNames[e.Name] {
				return fmt.Errorf("duplicate experiment name %q", e.Name)
			}
			expNames[e.Name] = true
			if err := noNUL(e.Name, e.Salt, e.Attribute); err != nil {
				return fmt.Errorf("experiment %q: %w", e.Name, err)
			}
			if e.BucketFrom < 0 || e.BucketTo > totalBuckets || e.BucketFrom >= e.BucketTo {
				return fmt.Errorf("experiment %q: invalid bucket range [%d,%d)", e.Name, e.BucketFrom, e.BucketTo)
			}
			for _, s := range segs {
				if e.BucketFrom < s.to && s.from < e.BucketTo {
					return fmt.Errorf("experiment %q: bucket range overlaps another experiment in group %q", e.Name, g.Name)
				}
			}
			segs = append(segs, seg{e.BucketFrom, e.BucketTo})
			if len(e.Allowed) > 0 && e.Attribute == "" {
				return fmt.Errorf("experiment %q: allowed values require an attribute", e.Name)
			}
			if len(e.Variants) == 0 {
				return fmt.Errorf("experiment %q: no variants", e.Name)
			}
			sum := 0
			varNames := map[string]bool{}
			for _, v := range e.Variants {
				if v.Name == "" {
					return fmt.Errorf("experiment %q: empty variant name", e.Name)
				}
				if varNames[v.Name] {
					return fmt.Errorf("experiment %q: duplicate variant %q", e.Name, v.Name)
				}
				varNames[v.Name] = true
				if v.Weight <= 0 {
					return fmt.Errorf("experiment %q: variant %q weight must be positive", e.Name, v.Name)
				}
				if err := noNUL(v.Name); err != nil {
					return fmt.Errorf("experiment %q: %w", e.Name, err)
				}
				sum += v.Weight
			}
			if sum != totalBuckets {
				return fmt.Errorf("experiment %q: variant weights sum to %d, want %d", e.Name, sum, totalBuckets)
			}
		}
	}
	return nil
}
