package connectors

import (
	"fmt"
	"time"
)

type ThrottleError struct {
	RetryAfter time.Duration
	Cause      error
}

func (e *ThrottleError) Error() string {
	return fmt.Sprintf("throttled: retry after %v (cause: %v)", e.RetryAfter, e.Cause)
}
