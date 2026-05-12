// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"strings"
)

// Gender records a person's gender per CCGGS §11.1. The core
// vocabulary is "male", "female", "unknown". Implementations MAY
// extend the vocabulary by registering additional values; any
// non-core value used MUST be documented by the deploying system.
type Gender string

const (
	// GenderMale is the core "male" value.
	GenderMale Gender = "male"
	// GenderFemale is the core "female" value.
	GenderFemale Gender = "female"
	// GenderUnknown is the core "unknown" value (recorded but unspecified).
	GenderUnknown Gender = "unknown"
	// GenderUnset means no gender has been recorded at all.
	// It is distinct from GenderUnknown ("recorded as unknown").
	GenderUnset Gender = ""
)

// IsCore reports whether g is one of the three core vocabulary values.
func (g Gender) IsCore() bool {
	switch g {
	case GenderMale, GenderFemale, GenderUnknown:
		return true
	default:
		return false
	}
}

// IsUnset reports whether no gender was recorded.
func (g Gender) IsUnset() bool { return g == GenderUnset }

// ParseGender normalizes and validates a gender string. Empty
// input returns GenderUnset, nil. Unknown values are rejected
// unless allowExtension is true, in which case they are returned
// as-is for the caller's extended vocabulary.
func ParseGender(s string, allowExtension bool) (Gender, error) {
	t := strings.ToLower(strings.TrimSpace(s))
	if t == "" {
		return GenderUnset, nil
	}
	g := Gender(t)
	if g.IsCore() {
		return g, nil
	}
	if allowExtension {
		return g, nil
	}
	return GenderUnset, fmt.Errorf("model: gender %q is not in the core vocabulary", s)
}
