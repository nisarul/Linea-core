// SPDX-License-Identifier: AGPL-3.0-or-later

package explain

import (
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/query"
	"github.com/nisarul/Linea-core/store"
)

// EdgeExplanation is the per-step explanation record.
type EdgeExplanation struct {
	// Index is the 0-based position of this step in the path.
	Index int
	// FromPerson and ToPerson identify the endpoints in
	// traversal order.
	FromPerson model.ID
	ToPerson   model.ID
	// FromIsUnknownAncestor / ToIsUnknownAncestor flag whether
	// either endpoint is an unknown-ancestor placeholder
	// (CCGGS §5.3). Consumers MUST surface this distinctly from
	// known persons.
	FromIsUnknownAncestor bool
	ToIsUnknownAncestor   bool
	// RelationshipID is the underlying edge.
	RelationshipID model.ID
	// Type is the relationship type (ParentChild or Marriage).
	Type model.RelationshipType
	// Direction reflects whether the edge was walked forward or reverse.
	Direction query.EdgeDirection
	// Certainty of the underlying edge.
	Certainty model.Certainty
	// Continuity carries Continuous/Gapped + gap size.
	Continuity model.Continuity
	// SourceIDs are the citations attached to this edge.
	SourceIDs []model.ID
	// IsWeakestLink is true for the edge whose certainty equals
	// the path's overall certainty AND no later edge is weaker.
	// (For paths with multiple edges sharing the minimum, only
	// the first such edge is flagged, for stable explanations.)
	IsWeakestLink bool
}

// PathExplanation is the structured explanation of a single Path.
type PathExplanation struct {
	From               model.ID
	To                 model.ID
	Length             int
	Classification     query.PathClassification
	OverallCertainty   model.Certainty
	TotalGapGenerations int
	GapEdgeCount       int
	Edges              []EdgeExplanation
	GraphVersion       store.Version
}

// CommonAncestorExplanation is the structured explanation of a
// NearestKnownCommonAncestor result.
type CommonAncestorExplanation struct {
	AncestorID         model.ID
	AncestorIsUnknown  bool
	TotalGenerations   int
	CombinedCertainty  model.Certainty
	GraphVersion       store.Version
	// PathFromA and PathFromB are the upward (descendant→ancestor)
	// explanation paths. Either may be empty if a == ancestor or
	// b == ancestor.
	PathFromA *PathExplanation
	PathFromB *PathExplanation
}
