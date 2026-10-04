package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// bucketOf hashes label, salt, object, userID joined by NUL bytes,
// interprets the first 8 digest bytes as big-endian uint64, mod 10000.
func bucketOf(label, salt, object, userID string) (int, error) {
	if err := noNUL(label, salt, object, userID); err != nil {
		return 0, err
	}
	h := sha256.New()
	for i, p := range []string{label, salt, object, userID} {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(p))
	}
	sum := h.Sum(nil)
	return int(binary.BigEndian.Uint64(sum[:8]) % totalBuckets), nil
}

type Decision struct {
	UserID        string `json:"user_id"`
	Group         string `json:"group"`
	GroupBucket   int    `json:"group_bucket"`
	Experiment    string `json:"experiment,omitempty"`
	VariantBucket int    `json:"variant_bucket,omitempty"`
	Variant       string `json:"variant,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

const (
	reasonNoExperiment     = "no_experiment"
	reasonAttributeMissing = "attribute_missing"
	reasonAttributeDenied  = "attribute_not_allowed"
)

// assignGroup decides a user's experiment and variant within one group.
func assignGroup(g *Group, userID string, attrs map[string]string) (Decision, error) {
	d := Decision{UserID: userID, Group: g.Name}
	gb, err := bucketOf("group", g.Salt, g.Name, userID)
	if err != nil {
		return d, err
	}
	d.GroupBucket = gb
	var hit *Experiment
	for i := range g.Experiments {
		e := &g.Experiments[i]
		if gb >= e.BucketFrom && gb < e.BucketTo {
			hit = e
			break
		}
	}
	if hit == nil {
		d.Reason = reasonNoExperiment
		return d, nil
	}
	if hit.Attribute != "" {
		val, ok := attrs[hit.Attribute]
		if !ok {
			d.Reason = reasonAttributeMissing
			return d, nil
		}
		allowed := false
		for _, a := range hit.Allowed {
			if val == a {
				allowed = true
				break
			}
		}
		if !allowed {
			d.Reason = reasonAttributeDenied
			return d, nil
		}
	}
	vb, err := bucketOf("variant", hit.Salt, hit.Name, userID)
	if err != nil {
		return d, err
	}
	d.Experiment = hit.Name
	d.VariantBucket = vb
	acc := 0
	for _, v := range hit.Variants {
		acc += v.Weight
		if vb < acc {
			d.Variant = v.Name
			break
		}
	}
	if d.Variant == "" {
		return d, fmt.Errorf("experiment %q: variant bucket %d out of range", hit.Name, vb)
	}
	return d, nil
}

func splitAttrs(header, row []string) (string, map[string]string, error) {
	if len(row) != len(header) {
		return "", nil, fmt.Errorf("row has %d fields, want %d", len(row), len(header))
	}
	attrs := map[string]string{}
	for i := 1; i < len(header); i++ {
		attrs[header[i]] = row[i]
	}
	return row[0], attrs, nil
}
