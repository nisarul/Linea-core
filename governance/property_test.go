// SPDX-License-Identifier: AGPL-3.0-or-later

package governance

import (
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/nisarul/Linea-core/model"
)

var propertyConfig = &quick.Config{
	MaxCount: 500,
	Rand:     rand.New(rand.NewSource(13)),
}

func anyState(r *rand.Rand) model.ProposalState {
	return model.ProposalState(1 + r.Intn(6))
}

// Property: terminal states never transition.
func TestProp_Terminal_NeverTransitions(t *testing.T) {
	f := func() bool {
		r := propertyConfig.Rand
		from := anyState(r)
		to := anyState(r)
		if !from.IsTerminal() {
			return true // skip non-terminal cases
		}
		return !CanTransition(from, to)
	}
	require.NoError(t, quick.Check(f, propertyConfig))
}

// Property: Withdraw is reachable from every non-terminal state.
func TestProp_Withdraw_AlwaysReachableFromNonTerminal(t *testing.T) {
	f := func() bool {
		r := propertyConfig.Rand
		from := anyState(r)
		if from.IsTerminal() {
			return true
		}
		return CanTransition(from, model.ProposalWithdrawn)
	}
	require.NoError(t, quick.Check(f, propertyConfig))
}

// Property: every transition that CanTransition allows is actually
// applied successfully by Transition (with required reason for Reject).
func TestProp_Transition_ConsistentWithCanTransition(t *testing.T) {
	f := func() bool {
		r := propertyConfig.Rand
		from := anyState(r)
		to := anyState(r)
		if from.IsTerminal() {
			return true
		}
		// Build a proposal in `from` state.
		p, err := model.NewProposal(model.NewID(),
			model.ProposalActionCreate, model.EntityKindPerson, model.ProposalOptions{})
		if err != nil {
			return false
		}
		if from != model.ProposalDraft {
			p = p.WithStateUnchecked(from, model.ProposalTransition{From: model.ProposalDraft, To: from})
		}
		reason := ""
		if to == model.ProposalRejected {
			reason = "x"
		}
		_, err = Transition(p, to, "actor", 1, reason)
		can := CanTransition(from, to)
		// either both succeed or both fail
		return (err == nil) == can
	}
	require.NoError(t, quick.Check(f, propertyConfig))
}
