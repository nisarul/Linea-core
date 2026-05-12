// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "fmt"

// ContinuityState expresses completeness of knowledge along an
// edge, per CCGGS §6.2.
type ContinuityState uint8

const (
	// ContinuityContinuous — direct edge, no missing generations.
	ContinuityContinuous ContinuityState = 1
	// ContinuityGapped — one or more intermediate generations are
	// known to be missing. Must be paired with a Continuity value
	// whose GapGenerations is set.
	ContinuityGapped ContinuityState = 2
)

// IsValid reports whether s is one of the two allowed states.
func (s ContinuityState) IsValid() bool {
	return s == ContinuityContinuous || s == ContinuityGapped
}

// String returns the spec-defined name of the state.
func (s ContinuityState) String() string {
	switch s {
	case ContinuityContinuous:
		return "Continuous"
	case ContinuityGapped:
		return "Gapped"
	default:
		return fmt.Sprintf("ContinuityState(%d)", uint8(s))
	}
}

// GapGenerations describes the size of a gap on a Gapped edge.
//
// A non-Known gap with KnownSize == false represents an
// unspecified-but-nonzero gap (CCGGS §6.2 "Unknown").
type GapGenerations struct {
	// KnownSize indicates whether the gap size is quantified.
	KnownSize bool
	// Size is the number of intermediate generations missing.
	// Valid only when KnownSize is true. Must be >= 1.
	Size int
}

// UnknownGap returns a GapGenerations marker for a gap whose
// size is not known.
func UnknownGap() GapGenerations { return GapGenerations{KnownSize: false} }

// KnownGap returns a GapGenerations of the given size.
// It returns an error if size < 1.
func KnownGap(size int) (GapGenerations, error) {
	if size < 1 {
		return GapGenerations{}, fmt.Errorf("model: known gap size must be >= 1, got %d", size)
	}
	return GapGenerations{KnownSize: true, Size: size}, nil
}

// String renders the gap for diagnostics.
func (g GapGenerations) String() string {
	if !g.KnownSize {
		return "Unknown"
	}
	return fmt.Sprintf("%d", g.Size)
}

// Continuity bundles the continuity state and (when Gapped) the
// gap size. It is the value attached to relationship edges.
type Continuity struct {
	State ContinuityState
	Gap   GapGenerations // meaningful only when State == ContinuityGapped
}

// NewContinuous returns a Continuous continuity value.
func NewContinuous() Continuity {
	return Continuity{State: ContinuityContinuous}
}

// NewGapped returns a Gapped continuity value with the supplied
// gap size. To represent a gap of unknown size, pass UnknownGap().
func NewGapped(gap GapGenerations) Continuity {
	return Continuity{State: ContinuityGapped, Gap: gap}
}

// IsValid reports whether the continuity is internally consistent.
func (c Continuity) IsValid() bool {
	if !c.State.IsValid() {
		return false
	}
	if c.State == ContinuityGapped && c.Gap.KnownSize && c.Gap.Size < 1 {
		return false
	}
	return true
}

// IsGapped reports whether the continuity represents a gap.
func (c Continuity) IsGapped() bool {
	return c.State == ContinuityGapped
}

// String renders the continuity for diagnostics.
func (c Continuity) String() string {
	if c.State == ContinuityGapped {
		return fmt.Sprintf("Gapped(%s)", c.Gap)
	}
	return c.State.String()
}
