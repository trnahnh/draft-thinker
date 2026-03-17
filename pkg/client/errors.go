package client

import "fmt"

type UpstreamError struct {
	StatusCode int
	Body       string
	Provider   string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("%s returned status %d: %s", e.Provider, e.StatusCode, e.Body)
}

type UpstreamTimeoutError struct {
	Provider string
}

func (e *UpstreamTimeoutError) Error() string {
	return fmt.Sprintf("timeout contacting %s", e.Provider)
}
