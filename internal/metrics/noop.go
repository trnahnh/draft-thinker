package metrics

import "time"

type NoopRecorder struct{}

func (n *NoopRecorder) RecordRequest(model string, status int)                {}
func (n *NoopRecorder) RecordUpstreamLatency(provider string, d time.Duration) {}
func (n *NoopRecorder) RecordError(errorType string)                           {}
func (n *NoopRecorder) RecordEntropy(value float64)                            {}
func (n *NoopRecorder) RecordRoutingDecision(decision string)                  {}
func (n *NoopRecorder) RecordSpeculativeTrigger()                              {}
func (n *NoopRecorder) RecordSpeculativeCancellation()                         {}
func (n *NoopRecorder) RecordSpeculativeLatencySaved(d time.Duration)          {}
