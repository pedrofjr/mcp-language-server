package main

import "time"

// Limites NFR v1.1+ (Release Readiness) — referência para testes e benchmarks.
const (
	NFRCriticalOperationTimeoutMax = 15 * time.Second
	NFRRepresentativeLoadP95Max    = 5 * time.Second
	NFROperationalErrorCoverageMin = 0.95
)
