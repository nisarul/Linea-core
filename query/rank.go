// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import "github.com/nisarul/Linea-core/model"

// Less reports whether a should be ranked before b.
//
// The comparison implements the 5-criteria lexicographic ordering
// from CCGGS §9.3:
//
//   1. Higher path certainty (Certain > Probable > Uncertain)
//   2. Smaller total gap size
//   3. Fewer gap edges
//   4. Shorter edge length
//   5. Stable deterministic tiebreaker by node-id sequence
func Less(a, b Path) bool {
	// (1) certainty: higher rank wins
	if a.Certainty.Rank() != b.Certainty.Rank() {
		return a.Certainty.Rank() > b.Certainty.Rank()
	}
	// (2) total gap
	if a.TotalGap != b.TotalGap {
		return a.TotalGap < b.TotalGap
	}
	// (3) gap-edge count
	if a.GapEdges != b.GapEdges {
		return a.GapEdges < b.GapEdges
	}
	// (4) length
	if a.Length != b.Length {
		return a.Length < b.Length
	}
	// (5) stable tiebreaker — lexicographic on the sequence of
	// node ids forming the path. We compare the node-id signature
	// element-wise; the signature length equals Length+1.
	as, bs := nodeSignature(a), nodeSignature(b)
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}
	return len(as) < len(bs)
}

// nodeSignature returns the ordered list of node IDs visited.
func nodeSignature(p Path) []model.ID {
	if len(p.Steps) == 0 {
		return nil
	}
	out := make([]model.ID, 0, len(p.Steps)+1)
	out = append(out, p.Steps[0].FromPerson)
	for _, s := range p.Steps {
		out = append(out, s.ToPerson)
	}
	return out
}

// computeAggregate computes Certainty/TotalGap/GapEdges/Length/
// Classification for a freshly-built sequence of Steps.
func computeAggregate(steps []Step) (model.Certainty, int, int, int, PathClassification) {
	if len(steps) == 0 {
		return model.CertaintyCertain, 0, 0, 0, PathLineage
	}
	cert := steps[0].Relationship.Certainty()
	totalGap := 0
	gapEdges := 0
	classification := PathLineage
	for _, s := range steps {
		c := s.Relationship.Certainty()
		if c.Rank() < cert.Rank() {
			cert = c
		}
		if s.Relationship.Continuity().IsGapped() {
			gapEdges++
		}
		// GapWeight returns 1 for continuous PC edges, gap+1 for
		// gapped, sentinel for unknown-gap, 0 for marriage. We
		// want only the gap component: subtract the base weight.
		weight := s.Relationship.GapWeight()
		if s.Relationship.Type() == model.RelTypeParentChild {
			if weight > 1 {
				totalGap += weight - 1
			}
		}
		if s.Relationship.Type() == model.RelTypeMarriage {
			classification = PathAffinal
		}
	}
	return cert, totalGap, gapEdges, len(steps), classification
}
