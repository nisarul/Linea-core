// SPDX-License-Identifier: AGPL-3.0-or-later

package governance

import (
	"fmt"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
)

// allowedTransitions maps each ProposalState to the set of states
// it may legally transition to, per CCGGS §8.3:
//
//	Draft → Submitted → UnderReview → (Accepted | Rejected | Withdrawn)
//
// Withdrawal is also allowed from Draft, Submitted, and
// UnderReview — author MAY withdraw at any time before terminal.
var allowedTransitions = map[model.ProposalState]map[model.ProposalState]bool{
	model.ProposalDraft: {
		model.ProposalSubmitted: true,
		model.ProposalWithdrawn: true,
	},
	model.ProposalSubmitted: {
		model.ProposalUnderReview: true,
		model.ProposalWithdrawn:   true,
	},
	model.ProposalUnderReview: {
		model.ProposalAccepted:  true,
		model.ProposalRejected:  true,
		model.ProposalWithdrawn: true,
	},
}

// CanTransition reports whether `to` is reachable from `from`
// under the proposal state machine.
func CanTransition(from, to model.ProposalState) bool {
	if !from.IsValid() || !to.IsValid() {
		return false
	}
	if from.IsTerminal() {
		return false
	}
	return allowedTransitions[from][to]
}

// Transition validates and applies a state transition to p.
// On success it returns a new Proposal with the updated state
// and a fresh history entry appended.
//
// Required arguments:
//   - to:        the new state.
//   - actor:     opaque identifier of who performed the transition.
//   - timestamp: unix epoch seconds at which the transition occurred.
//   - reason:    rationale; REQUIRED when `to == Rejected`.
//
// On invalid transitions or missing rejection reason it returns
// an *errors.Error with a stable Code.
func Transition(
	p model.Proposal,
	to model.ProposalState,
	actor string,
	timestamp int64,
	reason string,
) (model.Proposal, error) {
	from := p.State()
	if !CanTransition(from, to) {
		if from.IsTerminal() {
			return model.Proposal{}, lerrors.New(lerrors.CodeImmutableTerminalProposal,
				fmt.Sprintf("proposal %s is in terminal state %s", p.ID(), from))
		}
		return model.Proposal{}, lerrors.New(lerrors.CodeInvalidTransition,
			fmt.Sprintf("proposal %s: transition %s -> %s is not allowed", p.ID(), from, to))
	}
	if to == model.ProposalRejected && reason == "" {
		return model.Proposal{}, lerrors.New(lerrors.CodeInvalidArgument,
			fmt.Sprintf("proposal %s: rejection requires a non-empty reason", p.ID()))
	}
	tr := model.ProposalTransition{
		From:      from,
		To:        to,
		Actor:     actor,
		Timestamp: timestamp,
		Reason:    reason,
	}
	return p.WithStateUnchecked(to, tr), nil
}
