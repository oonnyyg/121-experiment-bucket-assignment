package main

import (
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{Groups: []Group{{
		Name: "g", Salt: "s",
		Experiments: []Experiment{{
			Name: "e", Salt: "es", BucketFrom: 0, BucketTo: 10000,
			Variants: []Variant{{Name: "a", Weight: 10000}},
		}},
	}}}
}

func TestValidateOK(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]func(*Config){
		"duplicate group": func(c *Config) { c.Groups = append(c.Groups, c.Groups[0]) },
		"duplicate experiment across groups": func(c *Config) {
			g2 := c.Groups[0]
			g2.Name = "g2"
			c.Groups = append(c.Groups, g2)
		},
		"overlap": func(c *Config) {
			e2 := c.Groups[0].Experiments[0]
			e2.Name = "e2"
			e2.BucketFrom = 5000
			c.Groups[0].Experiments = append(c.Groups[0].Experiments, e2)
		},
		"empty range":       func(c *Config) { c.Groups[0].Experiments[0].BucketTo = 0 },
		"range above 10000": func(c *Config) { c.Groups[0].Experiments[0].BucketTo = 10001 },
		"negative from":     func(c *Config) { c.Groups[0].Experiments[0].BucketFrom = -1 },
		"weights not 10000": func(c *Config) { c.Groups[0].Experiments[0].Variants[0].Weight = 9999 },
		"zero weight":       func(c *Config) { c.Groups[0].Experiments[0].Variants[0].Weight = 0 },
		"no variants":       func(c *Config) { c.Groups[0].Experiments[0].Variants = nil },
		"allowed without attribute": func(c *Config) {
			c.Groups[0].Experiments[0].Allowed = []string{"US"}
		},
		"NUL in salt": func(c *Config) { c.Groups[0].Salt = "a\x00b" },
	}
	for name, mutate := range cases {
		c := validConfig()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestValidateAdjacentRangesOK(t *testing.T) {
	c := validConfig()
	e2 := c.Groups[0].Experiments[0]
	e2.Name = "e2"
	c.Groups[0].Experiments[0].BucketTo = 5000
	e2.BucketFrom = 5000
	e2.BucketTo = 10000
	c.Groups[0].Experiments = append(c.Groups[0].Experiments, e2)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateErrorMessages(t *testing.T) {
	c := validConfig()
	c.Groups[0].Experiments[0].Variants[0].Weight = 5
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "10000") {
		t.Fatalf("unexpected error: %v", err)
	}
}
