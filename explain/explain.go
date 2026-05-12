// SPDX-License-Identifier: AGPL-3.0-or-later

package explain

import (
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/query"
	"github.com/nisarul/Linea-core/store"
)

// Path produces a structured explanation of a Path against the
// supplied snapshot. The snapshot is needed to look up
// per-person properties (such as the unknown-ancestor flag).
//
// The function does not allocate beyond what's necessary; it
// touches the store O(len(path.Steps)) times.
func Path(rtx store.ReadTx, p query.Path) (PathExplanation, error) {
	out := PathExplanation{
		From:                p.From(),
		To:                  p.To(),
		Length:              p.Length,
		Classification:      p.Classification,
		OverallCertainty:    p.Certainty,
		TotalGapGenerations: p.TotalGap,
		GapEdgeCount:        p.GapEdges,
		GraphVersion:        p.GraphVersion,
		Edges:               make([]EdgeExplanation, 0, len(p.Steps)),
	}

	// Identify the FIRST edge whose certainty == OverallCertainty.
	weakestIdx := -1
	for i, s := range p.Steps {
		if s.Relationship.Certainty() == p.Certainty {
			weakestIdx = i
			break
		}
	}

	for i, s := range p.Steps {
		fromUnknown, err := isUnknownPerson(rtx, s.FromPerson)
		if err != nil {
			return PathExplanation{}, err
		}
		toUnknown, err := isUnknownPerson(rtx, s.ToPerson)
		if err != nil {
			return PathExplanation{}, err
		}
		out.Edges = append(out.Edges, EdgeExplanation{
			Index:                 i,
			FromPerson:            s.FromPerson,
			ToPerson:              s.ToPerson,
			FromIsUnknownAncestor: fromUnknown,
			ToIsUnknownAncestor:   toUnknown,
			RelationshipID:        s.Relationship.ID(),
			Type:                  s.Relationship.Type(),
			Direction:             s.Direction,
			Certainty:             s.Relationship.Certainty(),
			Continuity:            s.Relationship.Continuity(),
			SourceIDs:             s.Relationship.Sources(),
			IsWeakestLink:         i == weakestIdx,
		})
	}
	return out, nil
}

// CommonAncestor produces a structured explanation of an NKCA result.
//
// Both PathFromA and PathFromB are derived from the underlying
// query.Path values inside the CommonAncestor. If either path
// has zero steps (the queried person *is* the NKCA), the
// corresponding explanation field is left nil.
func CommonAncestor(rtx store.ReadTx, ca *query.CommonAncestor) (CommonAncestorExplanation, error) {
	if ca == nil {
		return CommonAncestorExplanation{}, nil
	}
	out := CommonAncestorExplanation{
		AncestorID:         ca.AncestorID,
		AncestorIsUnknown:  ca.Unknown,
		TotalGenerations:   ca.TotalGenerations,
		CombinedCertainty:  ca.CombinedCertainty,
		GraphVersion:       ca.GraphVersion,
	}
	if len(ca.PathFromA.Steps) > 0 {
		pa, err := Path(rtx, ca.PathFromA)
		if err != nil {
			return CommonAncestorExplanation{}, err
		}
		out.PathFromA = &pa
	}
	if len(ca.PathFromB.Steps) > 0 {
		pb, err := Path(rtx, ca.PathFromB)
		if err != nil {
			return CommonAncestorExplanation{}, err
		}
		out.PathFromB = &pb
	}
	return out, nil
}

func isUnknownPerson(rtx store.ReadTx, id model.ID) (bool, error) {
	if id.IsZero() {
		return false, nil
	}
	p, err := rtx.GetPerson(id)
	if err != nil {
		return false, err
	}
	return p.IsUnknownAncestor(), nil
}
