// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package router

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/f01c5700/avalanchego/api/health"
	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/message"
	"github.com/f01c5700/avalanchego/proto/pb/p2p"
	"github.com/f01c5700/avalanchego/snow/networking/benchlist"
	"github.com/f01c5700/avalanchego/snow/networking/handler"
	"github.com/f01c5700/avalanchego/snow/networking/timeout"
	"github.com/f01c5700/avalanchego/utils/logging"
	"github.com/f01c5700/avalanchego/utils/set"
)

// Router routes consensus messages to the Handler of the consensus
// engine that the messages are intended for
type Router interface {
	ExternalHandler
	InternalHandler

	Initialize(
		nodeID ids.NodeID,
		log logging.Logger,
		timeouts timeout.Manager,
		shutdownTimeout time.Duration,
		criticalChains set.Set[ids.ID],
		sybilProtectionEnabled bool,
		trackedSubnets set.Set[ids.ID],
		onFatal func(exitCode int),
		healthConfig HealthConfig,
		reg prometheus.Registerer,
	) error
	Shutdown(context.Context)
	AddChain(ctx context.Context, chain handler.Handler)
	health.Checker
}

// InternalHandler deals with messages internal to this node
type InternalHandler interface {
	benchlist.Benchable

	RegisterRequest(
		ctx context.Context,
		nodeID ids.NodeID,
		chainID ids.ID,
		requestID uint32,
		op message.Op,
		failedMsg message.InboundMessage,
		engineType p2p.EngineType,
	)
}
