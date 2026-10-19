package principalruntime

import "errors"

// Only trusted Bot runtime dependency boundaries create this marker. Its text
// stays identical for existing callers, which retain their old error mapping.
type botPurgeDependencyFailure struct{ cause error }

func (e *botPurgeDependencyFailure) Error() string { return e.cause.Error() }
func (e *botPurgeDependencyFailure) Unwrap() error { return e.cause }

func IsBotPurgeDependencyUnavailable(err error) bool {
	var dependency *botPurgeDependencyFailure
	return errors.As(err, &dependency)
}
