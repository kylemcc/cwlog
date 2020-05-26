package writer

import (
	"errors"
	"time"
)

var (
	// errIgnore signals to retry that the returned error should
	// be ignored, the error count not incremented, and a retry
	// should be attempted immediately
	errIgnore = errors.New("ignore")
)

type unrecoverableError struct {
	error
}

// noRetry returns an error that signals to retry to return immediately
// and not make additional attempts
func noRetry(err error) error {
	return &unrecoverableError{
		err,
	}
}

func retry(f func() error) error {
	var (
		cnt int
		err error
	)

	for cnt < maxRetries {
		if cnt > 0 && err != errIgnore {
			time.Sleep(time.Duration(cnt) * 100 * time.Millisecond)
		}

		if err = f(); err == nil {
			return nil
		} else if u, ok := err.(*unrecoverableError); ok {
			return u.error
		}

		if err != errIgnore {
			cnt++
		}
	}

	return err
}
