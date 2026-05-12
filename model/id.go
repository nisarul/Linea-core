// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ID is the stable, unique, opaque identifier of a Linea entity
// (Person, Relationship, Source, Proposal, GraphVersion).
//
// IDs are case-sensitive and must be non-empty. The canonical
// generator returns RFC 4122 UUIDs, but any non-empty string is
// accepted so that ingest from external systems can preserve
// existing identifiers.
type ID string

// NewID returns a freshly generated random ID.
func NewID() ID {
	return ID(uuid.NewString())
}

// ParseID validates and returns an ID from an arbitrary string.
// It rejects empty and whitespace-only inputs.
func ParseID(s string) (ID, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return "", fmt.Errorf("model: ID must not be empty")
	}
	return ID(t), nil
}

// String returns the underlying string form of the ID.
func (i ID) String() string { return string(i) }

// IsZero reports whether the ID is the empty value.
func (i ID) IsZero() bool { return i == "" }
