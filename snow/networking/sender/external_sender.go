// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/message"
	"github.com/f01c5700/avalanchego/snow/engine/common"
	"github.com/f01c5700/avalanchego/subnets"
	"github.com/f01c5700/avalanchego/utils/set"
)

// ExternalSender sends consensus messages to other validators
// Right now this is implemented in the networking package
type ExternalSender interface {
	Send(
		msg message.OutboundMessage,
		config common.SendConfig,
		subnetID ids.ID,
		allower subnets.Allower,
	) set.Set[ids.NodeID]
}
