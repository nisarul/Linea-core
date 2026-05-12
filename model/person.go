// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"strings"
)

// Person is a node in the genealogical graph (CCGGS §4).
//
// A Person value carries its own validated invariants. Construct
// new Persons via NewPerson or NewUnknownAncestor; do not build
// the struct literally outside the package.
type Person struct {
	id      ID
	names   []Name
	gender  Gender
	birth   TimeRange
	death   TimeRange
	notes   string
	unknown bool // true => unknown-ancestor placeholder (CCGGS §5.3)
}

// PersonOptions carries optional fields for constructing a Person.
// All fields are optional; the only required input is the ID.
type PersonOptions struct {
	Names  []Name
	Gender Gender
	Birth  TimeRange
	Death  TimeRange
	Notes  string
}

// NewPerson constructs a validated, normal (non-placeholder) Person.
// At least one Name is required for a normal Person.
func NewPerson(id ID, opts PersonOptions) (Person, error) {
	if id.IsZero() {
		return Person{}, fmt.Errorf("model: Person ID is required")
	}
	if len(opts.Names) == 0 {
		return Person{}, fmt.Errorf("model: Person %s requires at least one name", id)
	}
	for i, n := range opts.Names {
		if strings.TrimSpace(n.Text) == "" {
			return Person{}, fmt.Errorf("model: Person %s name #%d has empty text", id, i)
		}
	}
	if opts.Gender != GenderUnset && !opts.Gender.IsCore() {
		// Non-core values are permitted but flagged here only if
		// they are obviously malformed (whitespace, control chars).
		if strings.TrimSpace(string(opts.Gender)) == "" {
			return Person{}, fmt.Errorf("model: Person %s has malformed gender", id)
		}
	}
	if !opts.Birth.IsZero() {
		if _, err := NewTimeRange(opts.Birth.Earliest, opts.Birth.Latest, opts.Birth.Calendar, opts.Birth.Circa); err != nil {
			return Person{}, fmt.Errorf("model: Person %s birth: %w", id, err)
		}
	}
	if !opts.Death.IsZero() {
		if _, err := NewTimeRange(opts.Death.Earliest, opts.Death.Latest, opts.Death.Calendar, opts.Death.Circa); err != nil {
			return Person{}, fmt.Errorf("model: Person %s death: %w", id, err)
		}
	}
	return Person{
		id:     id,
		names:  append([]Name(nil), opts.Names...),
		gender: opts.Gender,
		birth:  opts.Birth,
		death:  opts.Death,
		notes:  opts.Notes,
	}, nil
}

// NewUnknownAncestor constructs an unknown-ancestor placeholder
// per CCGGS §5.3. Such a node carries NO fabricated attributes:
// no name, no gender, no dates. It exists solely to anchor known
// sibling relationships when the shared parent is undocumented.
func NewUnknownAncestor(id ID) (Person, error) {
	if id.IsZero() {
		return Person{}, fmt.Errorf("model: unknown-ancestor ID is required")
	}
	return Person{
		id:      id,
		unknown: true,
	}, nil
}

// ID returns the person's stable identifier.
func (p Person) ID() ID { return p.id }

// IsUnknownAncestor reports whether this Person is an
// unknown-ancestor placeholder (CCGGS §5.3).
func (p Person) IsUnknownAncestor() bool { return p.unknown }

// Names returns a copy of the person's names.
func (p Person) Names() []Name {
	out := make([]Name, len(p.names))
	copy(out, p.names)
	return out
}

// PreferredName returns the first preferred name in the list, or
// the first name if none is marked preferred. For unknown
// placeholders it returns the empty Name.
func (p Person) PreferredName() Name {
	for _, n := range p.names {
		if n.Preferred {
			return n
		}
	}
	if len(p.names) > 0 {
		return p.names[0]
	}
	return Name{}
}

// Gender returns the recorded gender (may be GenderUnset).
func (p Person) Gender() Gender { return p.gender }

// Birth returns the recorded birth time range.
func (p Person) Birth() TimeRange { return p.birth }

// Death returns the recorded death time range.
func (p Person) Death() TimeRange { return p.death }

// Notes returns free-form descriptive notes (non-structural).
func (p Person) Notes() string { return p.notes }

// String renders the person for diagnostics.
func (p Person) String() string {
	if p.unknown {
		return fmt.Sprintf("Person<%s,unknown-ancestor>", p.id)
	}
	return fmt.Sprintf("Person<%s,%s>", p.id, p.PreferredName().Text)
}
