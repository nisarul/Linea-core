// SPDX-License-Identifier: AGPL-3.0-or-later

// Package errors defines semantic error codes used across the
// Linea engine. These codes are spec-level outcomes (e.g. the
// CCGGS §9.2 NO_KNOWN_CONNECTION result) and are presentation-
// layer-agnostic by design.
//
// All errors here are values, not strings, and may be compared
// with errors.Is.
package errors

import "errors"

// Code is a stable, spec-level identifier for a Linea outcome.
// New codes MUST be added here, never invented at call sites.
type Code string

const (
	// CodeNoKnownConnection corresponds to CCGGS §9.2: no valid
	// genealogical path exists between the queried persons.
	// This is an *outcome*, not a failure; callers MUST surface
	// it as a first-class result and MUST NOT imply that absence
	// of a known connection means absence of a connection.
	CodeNoKnownConnection Code = "NO_KNOWN_CONNECTION"

	// CodePersonNotFound — referenced person does not exist.
	CodePersonNotFound Code = "PERSON_NOT_FOUND"

	// CodeRelationshipNotFound — referenced relationship does not exist.
	CodeRelationshipNotFound Code = "RELATIONSHIP_NOT_FOUND"

	// CodeSourceNotFound — referenced source does not exist.
	CodeSourceNotFound Code = "SOURCE_NOT_FOUND"

	// CodeProposalNotFound — referenced proposal does not exist.
	CodeProposalNotFound Code = "PROPOSAL_NOT_FOUND"

	// CodeInvalidTransition — proposal state-machine violation.
	CodeInvalidTransition Code = "INVALID_PROPOSAL_TRANSITION"

	// CodeImmutableTerminalProposal — attempt to mutate a proposal
	// already in a terminal state.
	CodeImmutableTerminalProposal Code = "IMMUTABLE_TERMINAL_PROPOSAL"

	// CodeCycleDetected — applying the change would introduce a
	// parent-child cycle, which is forbidden by GGCFS §6.2.
	CodeCycleDetected Code = "CYCLE_DETECTED"

	// CodeForbiddenRelationship — the requested relationship type
	// is not one of the allowed primitives (CCGGS §5.1).
	CodeForbiddenRelationship Code = "FORBIDDEN_RELATIONSHIP"

	// CodeFabricationAttempt — an attempt was made to set
	// substantive attributes on an unknown-ancestor placeholder
	// (CCGGS §5.3).
	CodeFabricationAttempt Code = "FABRICATION_ATTEMPT"

	// CodeVersionNotFound — requested historical graph version
	// does not exist.
	CodeVersionNotFound Code = "VERSION_NOT_FOUND"

	// CodeInvalidArgument — generic input validation failure.
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
)

// Error is a Linea engine error carrying a stable Code, a
// human-readable message, and an optional wrapped cause.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

// New returns a new Error with the given Code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap returns a new Error wrapping the supplied cause.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Cause.Error()
	}
	return string(e.Code) + ": " + e.Message
}

// Unwrap returns the wrapped cause for use with errors.Is/As.
func (e *Error) Unwrap() error { return e.Cause }

// Is reports whether the target error has the same Code.
// Two *Error values match if their Codes are equal; this lets
// callers do errors.Is(err, errors.New(CodeX, "")).
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// HasCode reports whether err is a Linea Error with the given Code.
func HasCode(err error, code Code) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code == code
}

// IsNoKnownConnection is a convenience for the most common semantic outcome.
func IsNoKnownConnection(err error) bool {
	return HasCode(err, CodeNoKnownConnection)
}

// ErrNoKnownConnection is the canonical sentinel for the
// NO_KNOWN_CONNECTION outcome. Use errors.Is(err, ErrNoKnownConnection)
// to test for it.
var ErrNoKnownConnection = New(CodeNoKnownConnection, "no known genealogical connection exists")
