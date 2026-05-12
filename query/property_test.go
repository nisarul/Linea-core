// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"math/rand"
	"sort"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/nisarul/Linea-core/model"
)

var propertyConfig = &quick.Config{
	MaxCount: 500,
	Rand:     rand.New(rand.NewSource(7)),
}

// genCertainty mirrors model.genCertainty (kept private there).
func genCertainty(r *rand.Rand) model.Certainty {
	switch r.Intn(3) {
	case 0:
		return model.CertaintyCertain
	case 1:
		return model.CertaintyProbable
	}
	return model.CertaintyUncertain
}

// genPath returns a synthetic Path with random ranking dimensions.
// All node IDs are deterministic so the stable tiebreaker bites.
func genPath(r *rand.Rand) Path {
	length := 1 + r.Intn(5)
	steps := make([]Step, length)
	for i := 0; i < length; i++ {
		steps[i] = Step{
			FromPerson: model.ID(randID(r)),
			ToPerson:   model.ID(randID(r)),
		}
	}
	return Path{
		Steps:          steps,
		Length:         length,
		Certainty:      genCertainty(r),
		TotalGap:       r.Intn(10),
		GapEdges:       r.Intn(5),
		Classification: PathLineage,
	}
}

func randID(r *rand.Rand) string {
	const alphabet = "abcdef0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(b)
}

// Property: Less defines a strict weak ordering — for any pair
// (a, b) at most one of Less(a,b) and Less(b,a) is true; Less(a,a)
// is always false. This is what sort.SliceStable requires.
func TestProp_Less_StrictWeakOrdering(t *testing.T) {
	f := func() bool {
		r := propertyConfig.Rand
		a, b := genPath(r), genPath(r)
		ab, ba := Less(a, b), Less(b, a)
		// asymmetric:
		if ab && ba {
			return false
		}
		// irreflexive:
		if Less(a, a) || Less(b, b) {
			return false
		}
		return true
	}
	require.NoError(t, quick.Check(f, propertyConfig))
}

// Property: Less is transitive.
func TestProp_Less_Transitive(t *testing.T) {
	f := func() bool {
		r := propertyConfig.Rand
		a, b, c := genPath(r), genPath(r), genPath(r)
		if Less(a, b) && Less(b, c) {
			return Less(a, c)
		}
		return true
	}
	require.NoError(t, quick.Check(f, propertyConfig))
}

// Property: sorting twice produces the same order (stable + total).
func TestProp_Less_SortIdempotent(t *testing.T) {
	f := func() bool {
		r := propertyConfig.Rand
		n := 5 + r.Intn(10)
		paths := make([]Path, n)
		for i := range paths {
			paths[i] = genPath(r)
		}
		first := append([]Path(nil), paths...)
		sort.SliceStable(first, func(i, j int) bool { return Less(first[i], first[j]) })
		second := append([]Path(nil), first...)
		sort.SliceStable(second, func(i, j int) bool { return Less(second[i], second[j]) })
		for i := range first {
			if !pathsRankEqual(first[i], second[i]) {
				return false
			}
		}
		return true
	}
	require.NoError(t, quick.Check(f, propertyConfig))
}

func pathsRankEqual(a, b Path) bool {
	if a.Certainty != b.Certainty || a.TotalGap != b.TotalGap ||
		a.GapEdges != b.GapEdges || a.Length != b.Length {
		return false
	}
	if len(a.Steps) != len(b.Steps) {
		return false
	}
	for i := range a.Steps {
		if a.Steps[i].FromPerson != b.Steps[i].FromPerson ||
			a.Steps[i].ToPerson != b.Steps[i].ToPerson {
			return false
		}
	}
	return true
}
