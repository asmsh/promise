package promise

import "github.com/asmsh/promise/internal/status"

// Status holds the state and the fate info of the corresponding Promise,
// at the time it is created.
type Status uint32

// State returns the state of the promise.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) State() string {
	if status.IsStatePending(uint32(s)) {
		return "pending"
	} else if status.IsStateFulfilled(uint32(s)) {
		return "fulfilled"
	} else if status.IsStateRejected(uint32(s)) {
		return "rejected"
	} else if status.IsStatePanicked(uint32(s)) {
		return "panicked"
	}
	// only user-created Status values may result in reaching this
	return "<Unknown Promise State>"
}

// IsPending returns true, only if the state of the promise is Pending.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) IsPending() bool {
	return status.IsStatePending(uint32(s))
}

// IsFulfilled returns true, only if the state of the promise is Fulfilled.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) IsFulfilled() bool {
	return status.IsStateFulfilled(uint32(s))
}

// IsRejected returns true, only if the state of the promise is Rejected.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) IsRejected() bool {
	return status.IsStateRejected(uint32(s))
}

// IsPanicked returns true, only if the state of the promise is Panicked.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) IsPanicked() bool {
	return status.IsStatePanicked(uint32(s))
}

// Fate returns the fate of the promise.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) Fate() string {
	if status.IsFateHandled(uint32(s)) {
		return "handled"
	} else if status.IsFateResolved(uint32(s)) {
		return "resolved"
	} else if status.IsFateResolving(uint32(s)) || status.IsFateUnresolved(uint32(s)) {
		return "unresolved"
	}
	// only user-created Status values may result in reaching this
	return "<Unknown Promise Fate>"
}

// IsResolved returns true, only if the fate of the promise is Resolved or
// Handled.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) IsResolved() bool {
	return status.IsFateResolved(uint32(s)) || status.IsFateHandled(uint32(s))
}

// IsResolved returns true, only if the fate of the promise is Handled.
//
// For more info, see 'States and Fates' in the package comment.
func (s Status) IsHandled() bool {
	return status.IsFateHandled(uint32(s))
}
