package metrics

import "time"

type Recorder interface {
	RecordRequest(model string, status int)
	RecordUpstreamLatency(provider string, duration time.Duration)
	RecordError(errorType string)
	RecordEntropy(value float64)
	RecordRoutingDecision(decision string)
}
