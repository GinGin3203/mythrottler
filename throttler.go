package mythrottler

import (
	"errors"
	"golang.org/x/time/rate"
	"net/http"
	"regexp"
	"time"
)

type ThrottlingWrapper struct {
	transport                                 http.RoundTripper
	limiter                                   *rate.Limiter
	httpMethods                               map[string]struct{}
	pathPrefixRegexp, ignoredPathPrefixRegexp *regexp.Regexp
	allowOverqueue                            bool
}

func NewThrottler(
	transport http.RoundTripper,
	reqsPerUnit int,
	unitOfTime time.Duration,
	httpMethods, urlPrefixes, ignoredUrlPrefixes []string,
	allowOverqueue bool) (tw *ThrottlingWrapper, err error) {

	methods := make(map[string]struct{})
	for _, m := range httpMethods {
		methods[m] = struct{}{}
	}
	var pmr, pmri *regexp.Regexp
	if pmr, err = prefixMatcher(urlPrefixes); err != nil {
		return nil, err
	}

	if pmri, err = prefixMatcherIgnored(ignoredUrlPrefixes); err != nil {
		return nil, err
	}

	// "rate.Limit is represented as number of events per second.", поэтому
	// конвертируем requests per unit в requests per second
	requestRatePerSecond := float64(reqsPerUnit) * float64(time.Second/unitOfTime)

	tw = &ThrottlingWrapper{
		transport:               transport,
		limiter:                 rate.NewLimiter(rate.Limit(requestRatePerSecond), 1),
		pathPrefixRegexp:        pmr,
		ignoredPathPrefixRegexp: pmri,
		allowOverqueue:          allowOverqueue,
	}

	return tw, nil
}

var OverqueueDisallowedError = errors.New("unable to make request: over-queueing disallowed")

func (t *ThrottlingWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	lmt := t.limiter
	if !t.shouldWait(req) {
		return t.transport.RoundTrip(req)
	}

	if !(lmt.Allow() || t.allowOverqueue) {
		return nil, OverqueueDisallowedError
	}

	waitTime := lmt.
		Reserve().
		Delay()

	select {
	case <-time.After(waitTime):
		return t.transport.RoundTrip(req)
	}
}
