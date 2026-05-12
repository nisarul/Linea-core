// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"strings"
)

// SourceType classifies a citation source per CCGGS §7.1.
type SourceType string

const (
	// SourceTypePrimary is a primary historical source.
	SourceTypePrimary SourceType = "primary"
	// SourceTypeSecondary is a secondary scholarly source.
	SourceTypeSecondary SourceType = "secondary"
	// SourceTypeOral is oral tradition / oral history.
	SourceTypeOral SourceType = "oral"
	// SourceTypeDerived is a source derived from other sources
	// (e.g. genealogical compilations).
	SourceTypeDerived SourceType = "derived"
	// SourceTypeOther covers anything outside the core vocabulary.
	SourceTypeOther SourceType = "other"
)

// IsCore reports whether t is a core SourceType.
func (t SourceType) IsCore() bool {
	switch t {
	case SourceTypePrimary, SourceTypeSecondary, SourceTypeOral,
		SourceTypeDerived, SourceTypeOther:
		return true
	}
	return false
}

// Source is a first-class citation entity (CCGGS §7).
//
// Sources are referenced by ID from Relationships, Persons (via
// attachment records), and Proposals. Conflicting sources are
// represented by attaching multiple Sources to the same target;
// the engine never auto-resolves conflicts.
type Source struct {
	id       ID
	srcType  SourceType
	citation string // free-form, human-readable
	author   string
	title    string
	date     string // free-form (calendar / format varies)
	locator  string // page, folio, URL, etc.
	notes    string
}

// SourceOptions carries optional fields for NewSource.
type SourceOptions struct {
	Author  string
	Title   string
	Date    string
	Locator string
	Notes   string
}

// NewSource constructs a validated Source. Citation text is
// required. SourceType "" is normalised to SourceTypeOther.
func NewSource(id ID, t SourceType, citation string, opts SourceOptions) (Source, error) {
	if id.IsZero() {
		return Source{}, fmt.Errorf("model: Source ID is required")
	}
	if strings.TrimSpace(citation) == "" {
		return Source{}, fmt.Errorf("model: Source %s citation must not be empty", id)
	}
	if t == "" {
		t = SourceTypeOther
	}
	// Non-core types are allowed but obvious junk is rejected.
	if !t.IsCore() && strings.TrimSpace(string(t)) == "" {
		return Source{}, fmt.Errorf("model: Source %s has malformed type", id)
	}
	return Source{
		id:       id,
		srcType:  t,
		citation: citation,
		author:   opts.Author,
		title:    opts.Title,
		date:     opts.Date,
		locator:  opts.Locator,
		notes:    opts.Notes,
	}, nil
}

// ID returns the source's stable identifier.
func (s Source) ID() ID { return s.id }

// Type returns the source type.
func (s Source) Type() SourceType { return s.srcType }

// Citation returns the human-readable citation text.
func (s Source) Citation() string { return s.citation }

// Author returns the author (optional).
func (s Source) Author() string { return s.author }

// Title returns the title (optional).
func (s Source) Title() string { return s.title }

// Date returns the publication / origin date (optional, free-form).
func (s Source) Date() string { return s.date }

// Locator returns the page / folio / URL locator (optional).
func (s Source) Locator() string { return s.locator }

// Notes returns free-form notes (optional).
func (s Source) Notes() string { return s.notes }

// String renders the source for diagnostics.
func (s Source) String() string {
	return fmt.Sprintf("Source<%s,%s>", s.id, s.srcType)
}
