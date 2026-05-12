// SPDX-License-Identifier: AGPL-3.0-or-later

package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorFormatAndUnwrap(t *testing.T) {
	cause := stderrors.New("disk gone")
	e := Wrap(CodeVersionNotFound, "snapshot 42", cause)
	require.Equal(t, "VERSION_NOT_FOUND: snapshot 42: disk gone", e.Error())
	require.ErrorIs(t, e, cause)
}

func TestIsByCode(t *testing.T) {
	e := New(CodeNoKnownConnection, "")
	require.True(t, IsNoKnownConnection(e))
	require.True(t, HasCode(e, CodeNoKnownConnection))
	require.False(t, HasCode(e, CodeCycleDetected))
	require.True(t, stderrors.Is(e, ErrNoKnownConnection))
}

func TestIsAcrossWrapping(t *testing.T) {
	cause := New(CodeCycleDetected, "self-ancestor")
	wrapped := Wrap(CodeInvalidArgument, "validation failed", cause)
	require.True(t, HasCode(wrapped, CodeInvalidArgument))
	// Inner code should also be reachable via errors.As / Is on the wrapped chain.
	require.True(t, stderrors.Is(wrapped, cause))
}
