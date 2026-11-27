package ai

import "sync/atomic"

type PipelineStats struct {
	Enabled      bool   `json:"enabled"`
	L2Enabled    bool   `json:"l2_enabled"`
	L2Provider   string `json:"l2_provider"`
	L3Provider   string `json:"l3_provider"`
	L1Filtered   int64  `json:"l1_filtered"`
	L2Skipped    int64  `json:"l2_skipped"`
	L2Passed     int64  `json:"l2_passed"`
	L2Filtered   int64  `json:"l2_filtered"`
	L2BypassedL3 int64  `json:"l2_bypassed_l3"`
	L2Errors     int64  `json:"l2_errors"`
	L3Called     int64  `json:"l3_called"`
	L3Kept       int64  `json:"l3_kept"`
	L3Filtered   int64  `json:"l3_filtered"`
	L3Fallback   int64  `json:"l3_fallback"`
}

type pipelineCounters struct {
	l1Filtered   atomic.Int64
	l2Skipped    atomic.Int64
	l2Passed     atomic.Int64
	l2Filtered   atomic.Int64
	l2BypassedL3 atomic.Int64
	l2Errors     atomic.Int64
	l3Called     atomic.Int64
	l3Kept       atomic.Int64
	l3Filtered   atomic.Int64
	l3Fallback   atomic.Int64
}

func (owner *Evaluator) Stats() PipelineStats {
	e := owner.state.Load()
	s := PipelineStats{
		Enabled: e.provider != nil, L2Enabled: e.decision != nil,
		L1Filtered:   e.counters.l1Filtered.Load(),
		L2Skipped:    e.counters.l2Skipped.Load(),
		L2Passed:     e.counters.l2Passed.Load(),
		L2Filtered:   e.counters.l2Filtered.Load(),
		L2BypassedL3: e.counters.l2BypassedL3.Load(),
		L2Errors:     e.counters.l2Errors.Load(),
		L3Called:     e.counters.l3Called.Load(),
		L3Kept:       e.counters.l3Kept.Load(),
		L3Filtered:   e.counters.l3Filtered.Load(),
		L3Fallback:   e.counters.l3Fallback.Load(),
	}
	if e.provider != nil {
		s.L3Provider = e.provider.Name()
	}
	if e.decision != nil {
		s.L2Provider = e.decision.Name()
	}
	return s
}
