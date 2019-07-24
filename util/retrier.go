package util

import (
	"context"
	"errors"
	"net/http"
	"syscall"
	"time"

	"github.com/olivere/elastic/v7"
)

// Retrier is a custom Retry implementation.
type Retrier struct {
	backoff elastic.Backoff
}

// NewRetrier returns a new retrier with exponential backoff strategy.
func NewRetrier() *Retrier {
	return &Retrier{
		elastic.NewExponentialBackoff(10*time.Millisecond, 8*time.Millisecond),
	}
}

// Retry is a custom retry implementation.
func (r *Retrier) Retry(ctx context.Context, retry int, req *http.Request, resp *http.Response, err error) (time.Duration, bool, error) {
	// Fail hard on a specific error
	if err == syscall.ECONNREFUSED {
		return 0, false, errors.New("Elasticsearch or network down")
	}

	// Stop after 5 retries
	if retry >= 5 {
		return 0, false, nil
	}

	// Let the backoff strategy decide how long to wait and whether to stop
	wait, stop := r.backoff.Next(retry)
	return wait, stop, nil
}
