// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"strings"
)

// NameType classifies the role a Name plays for a Person.
// The core set follows CCGGS §4.2; deployments MAY extend it.
type NameType string

const (
	// NameTypeGiven is a given/first name.
	NameTypeGiven NameType = "given"
	// NameTypeFamily is a surname / family name.
	NameTypeFamily NameType = "family"
	// NameTypeFull is a full single-string name.
	NameTypeFull NameType = "full"
	// NameTypeKunyah is a teknonym (e.g. "Abu X").
	NameTypeKunyah NameType = "kunyah"
	// NameTypeTitle is an honorific or title.
	NameTypeTitle NameType = "title"
	// NameTypeEpithet is a descriptive epithet.
	NameTypeEpithet NameType = "epithet"
	// NameTypeOther covers any non-core type the deployment uses.
	NameTypeOther NameType = "other"
)

// Name is one of possibly many names associated with a Person.
// Linea makes no assumption that any single name is "the" name.
type Name struct {
	// Text is the name as written. Required, non-empty after trim.
	Text string
	// Language is an IETF BCP 47 / ISO 639 code (e.g. "en", "ar").
	// Empty means unspecified.
	Language string
	// Script is an ISO 15924 script code (e.g. "Latn", "Arab").
	// Empty means unspecified.
	Script string
	// Type classifies the name. Empty defaults to NameTypeFull.
	Type NameType
	// Preferred indicates this name is the preferred display form
	// for its language. At most one preferred per language is
	// expected; Linea does not enforce this.
	Preferred bool
}

// NewName constructs a validated Name.
func NewName(text, language, script string, t NameType, preferred bool) (Name, error) {
	tt := strings.TrimSpace(text)
	if tt == "" {
		return Name{}, fmt.Errorf("model: name text must not be empty")
	}
	if t == "" {
		t = NameTypeFull
	}
	return Name{
		Text:      tt,
		Language:  strings.TrimSpace(language),
		Script:    strings.TrimSpace(script),
		Type:      t,
		Preferred: preferred,
	}, nil
}

// String renders the name for diagnostics.
func (n Name) String() string {
	var b strings.Builder
	b.WriteString(n.Text)
	if n.Language != "" {
		b.WriteString(" [")
		b.WriteString(n.Language)
		if n.Script != "" {
			b.WriteString("/")
			b.WriteString(n.Script)
		}
		b.WriteString("]")
	}
	return b.String()
}
