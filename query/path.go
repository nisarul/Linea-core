// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// EdgeDirection describes how a Relationship is traversed in a
// path, since the same stored edge can be walked either way.
type EdgeDirection uint8

const (
	// EdgeForward — traverse from Relationship.From() to Relationship.To().
	EdgeForward EdgeDirection = 1
	// EdgeReverse — traverse from Relationship.To() to Relationship.From().
	EdgeReverse EdgeDirection = 2
)

// String returns a short label for diagnostics.
func (d EdgeDirection) String() string {
	switch d {
	case EdgeForward:
		return "->"
	case EdgeReverse:
		return "<-"
	}
	return "?"
}

// Step is a single traversal step along a path.
type Step struct {
	// Relationship is the underlying edge.
	Relationship model.Relationship
	// Direction is how the edge was walked in this path.
	Direction EdgeDirection
	// FromPerson is the person at the start of this step.
	FromPerson model.ID
	// ToPerson is the person at the end of this step.
	ToPerson model.ID
}

// Kind classifies a single step's contribution to the path.
type Kind uint8

const (
	// KindLineage — a Parent→Child edge (in either direction).
	KindLineage Kind = 1
	// KindAffinal — a Marriage edge.
	KindAffinal Kind = 2
)

// Kind returns whether this step is lineage or affinal.
func (s Step) Kind() Kind {
	if s.Relationship.Type() == model.RelTypeMarriage {
		return KindAffinal
	}
	return KindLineage
}

// PathClassification reports the overall kind of a Path.
type PathClassification uint8

const (
	// PathLineage contains only Parent→Child edges.
	PathLineage PathClassification = 1
	// PathAffinal contains at least one Marriage edge.
	PathAffinal PathClassification = 2
)

// String returns a short label for diagnostics.
func (c PathClassification) String() string {
	switch c {
	case PathLineage:
		return "lineage"
	case PathAffinal:
		return "affinal"
	}
	return "unknown"
}

// Path is a connected sequence of Steps from a start person to
// an end person. The start person is Steps[0].FromPerson; the
// end person is Steps[len-1].ToPerson.
type Path struct {
	// Steps in traversal order, length >= 1.
	Steps []Step
	// Certainty is the weakest-link certainty across all Steps
	// (CCGGS §6.1, §9.4).
	Certainty model.Certainty
	// TotalGap is the sum of GapGenerations across all Gapped
	// edges. Edges with unknown-size gaps contribute the
	// per-edge sentinel weight (see model.Relationship.GapWeight).
	TotalGap int
	// GapEdges is the number of Gapped edges in the path.
	GapEdges int
	// Length is the number of edges (== len(Steps)).
	Length int
	// Classification is lineage if every step is Parent→Child,
	// otherwise affinal.
	Classification PathClassification
	// GraphVersion is the version of the graph the path was
	// computed against (CCGGS §8.5).
	GraphVersion store.Version
}

// From returns the path's start person.
func (p Path) From() model.ID {
	if len(p.Steps) == 0 {
		return ""
	}
	return p.Steps[0].FromPerson
}

// To returns the path's end person.
func (p Path) To() model.ID {
	if len(p.Steps) == 0 {
		return ""
	}
	return p.Steps[len(p.Steps)-1].ToPerson
}

// PassesThrough reports whether the given person id appears
// anywhere on the path (as start, end, or intermediate node).
func (p Path) PassesThrough(id model.ID) bool {
	if len(p.Steps) == 0 {
		return false
	}
	if p.Steps[0].FromPerson == id {
		return true
	}
	for _, s := range p.Steps {
		if s.ToPerson == id {
			return true
		}
	}
	return false
}
