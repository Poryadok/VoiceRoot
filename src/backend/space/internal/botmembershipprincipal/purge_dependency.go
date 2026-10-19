package botmembershipprincipal

// dependencyUnavailable preserves a trusted transport/storage failure through
// principal.VerifyService's %w wrappers. Only the new handoff route classifies
// it; unknown KIDs, bad credentials and duplicate JTIs remain ordinary errors.
type dependencyUnavailable struct{ cause error }

func (e *dependencyUnavailable) Error() string { return e.cause.Error() }
func (e *dependencyUnavailable) Unwrap() error { return e.cause }
