// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import "github.com/f01c5700/avalanchego/trace"

const (
	DebugTrace TraceLevel = iota - 1
	InfoTrace             // Default
	NoTrace
)

type TraceLevel int

func getTracerIfEnabled(level, minLevel TraceLevel, tracer trace.Tracer) trace.Tracer {
	if level <= minLevel {
		return tracer
	}
	return trace.Noop
}
