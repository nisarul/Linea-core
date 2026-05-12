// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"
)

// All property tests use a deterministic seed so failures are
// reproducible. Override via -test.run with an explicit seed via
// the environment if you want to reproduce a specific sequence.
var propertyTestConfig = &quick.Config{
	MaxCount: 500,
	Rand:     rand.New(rand.NewSource(1)),
}

// genCertainty returns a generator for valid Certainty values.
func genCertainty(r *rand.Rand) Certainty {
	switch r.Intn(3) {
	case 0:
		return CertaintyCertain
	case 1:
		return CertaintyProbable
	}
	return CertaintyUncertain
}

// Property: CombineCertainties is commutative.
func TestProp_Certainty_Commutative(t *testing.T) {
	f := func() bool {
		r := propertyTestConfig.Rand
		a, b, c := genCertainty(r), genCertainty(r), genCertainty(r)
		return CombineCertainties(a, b, c) == CombineCertainties(c, b, a) &&
			CombineCertainties(a, b, c) == CombineCertainties(b, c, a)
	}
	require.NoError(t, quick.Check(f, propertyTestConfig))
}

// Property: CombineCertainties is associative.
func TestProp_Certainty_Associative(t *testing.T) {
	f := func() bool {
		r := propertyTestConfig.Rand
		a, b, c := genCertainty(r), genCertainty(r), genCertainty(r)
		return a.Min(b).Min(c) == a.Min(b.Min(c))
	}
	require.NoError(t, quick.Check(f, propertyTestConfig))
}

// Property: combining a value with itself is idempotent.
func TestProp_Certainty_Idempotent(t *testing.T) {
	f := func() bool {
		r := propertyTestConfig.Rand
		c := genCertainty(r)
		return CombineCertainties(c, c) == c && CombineCertainties(c, c, c) == c
	}
	require.NoError(t, quick.Check(f, propertyTestConfig))
}

// Property: combining never produces a value stronger than any input.
func TestProp_Certainty_NeverStrengthens(t *testing.T) {
	f := func() bool {
		r := propertyTestConfig.Rand
		a, b, c := genCertainty(r), genCertainty(r), genCertainty(r)
		out := CombineCertainties(a, b, c)
		minRank := a.Rank()
		if b.Rank() < minRank {
			minRank = b.Rank()
		}
		if c.Rank() < minRank {
			minRank = c.Rank()
		}
		return out.Rank() == minRank
	}
	require.NoError(t, quick.Check(f, propertyTestConfig))
}
