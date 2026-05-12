// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "fmt"

// Certainty expresses confidence in a claim, per CCGGS §6.1.
//
// The three allowed values have a normative ordering used by the
// weakest-link algebra: Certain (3) > Probable (2) > Uncertain (1).
type Certainty uint8

const (
	// CertaintyUncertain — asserted but weakly supported, contested,
	// or speculative. Rank 1.
	CertaintyUncertain Certainty = 1
	// CertaintyProbable — inferred from credible evidence;
	// reasonable scholarly consensus. Rank 2.
	CertaintyProbable Certainty = 2
	// CertaintyCertain — directly attested by primary or strong
	// corroborated sources. Rank 3.
	CertaintyCertain Certainty = 3
)

// Rank returns the integer rank of the certainty (1, 2, or 3).
func (c Certainty) Rank() int { return int(c) }

// IsValid reports whether c is one of the three allowed values.
func (c Certainty) IsValid() bool {
	return c == CertaintyUncertain || c == CertaintyProbable || c == CertaintyCertain
}

// String returns the spec-defined name of the certainty value.
func (c Certainty) String() string {
	switch c {
	case CertaintyCertain:
		return "Certain"
	case CertaintyProbable:
		return "Probable"
	case CertaintyUncertain:
		return "Uncertain"
	default:
		return fmt.Sprintf("Certainty(%d)", uint8(c))
	}
}

// Min returns the weaker (lower-ranked) of two certainties.
// This is the weakest-link combinator defined in CCGGS §6.1.
func (c Certainty) Min(other Certainty) Certainty {
	if other.Rank() < c.Rank() {
		return other
	}
	return c
}

// CombineCertainties applies the weakest-link combinator across
// any number of certainties. It panics on an empty slice; callers
// should not invoke it on empty inputs.
//
// CombineCertainties is associative and commutative.
func CombineCertainties(cs ...Certainty) Certainty {
	if len(cs) == 0 {
		panic("model: CombineCertainties requires at least one value")
	}
	out := cs[0]
	for _, c := range cs[1:] {
		out = out.Min(c)
	}
	return out
}
