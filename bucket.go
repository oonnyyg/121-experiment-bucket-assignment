package main

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	labelGroup   = "group"
	labelVariant = "variant"
)

// bucket computes SHA-256 over label, salt, object, userID joined by NUL
// bytes, interprets the first 8 digest bytes as a big-endian uint64 and
// returns it modulo totalBuckets.
func bucket(label, salt, object, userID string) int {
	h := sha256.New()
	for _, part := range []string{label, salt, object, userID} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return int(binary.BigEndian.Uint64(sum[:8]) % totalBuckets)
}

const (
	ReasonAssigned          = ""
	ReasonBucketNotCovered  = "bucket_not_covered"
	ReasonAttributeMismatch = "attribute_mismatch"
)

type Decision struct {
	Group         string `json:"group"`
	GroupBucket   int    `json:"group_bucket"`
	Experiment    string `json:"experiment,omitempty"`
	Variant       string `json:"variant,omitempty"`
	VariantBucket *int   `json:"variant_bucket,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// decideGroup assigns a user to at most one experiment within a group.
func decideGroup(g Group, userID string, attrs map[string]string) Decision {
	d := Decision{Group: g.Name, GroupBucket: bucket(labelGroup, g.Salt, g.Name, userID)}
	var hit *Experiment
	for i := range g.Experiments {
		e := &g.Experiments[i]
		if d.GroupBucket >= e.Start && d.GroupBucket < e.End {
			hit = e
			break
		}
	}
	if hit == nil {
		d.Reason = ReasonBucketNotCovered
		return d
	}
	if hit.Attribute != nil {
		v, ok := attrs[hit.Attribute.Key]
		matched := false
		if ok {
			for _, want := range hit.Attribute.Values {
				if v == want {
					matched = true
					break
				}
			}
		}
		if !matched {
			d.Reason = ReasonAttributeMismatch
			return d
		}
	}
	vb := bucket(labelVariant, hit.Salt, hit.Name, userID)
	d.Experiment = hit.Name
	d.VariantBucket = &vb
	acc := 0
	for _, v := range hit.Variants {
		acc += v.Weight
		if vb < acc {
			d.Variant = v.Name
			break
		}
	}
	return d
}
