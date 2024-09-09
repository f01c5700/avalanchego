// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"sync"
	"sync/atomic"

	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/snow/networking/router"
	"github.com/f01c5700/avalanchego/snow/validators"
	"github.com/f01c5700/avalanchego/utils/constants"
	"github.com/f01c5700/avalanchego/version"
)

var _ router.Router = (*beaconManager)(nil)

type beaconManager struct {
	router.Router
	beacons                     validators.Manager
	requiredConns               int64
	numConns                    int64
	onSufficientlyConnected     chan struct{}
	onceOnSufficientlyConnected sync.Once
}

func (b *beaconManager) Connected(nodeID ids.NodeID, nodeVersion *version.Application, subnetID ids.ID) {
	_, isBeacon := b.beacons.GetValidator(constants.PrimaryNetworkID, nodeID)
	if isBeacon &&
		constants.PrimaryNetworkID == subnetID &&
		atomic.AddInt64(&b.numConns, 1) >= b.requiredConns {
		b.onceOnSufficientlyConnected.Do(func() {
			close(b.onSufficientlyConnected)
		})
	}
	b.Router.Connected(nodeID, nodeVersion, subnetID)
}

func (b *beaconManager) Disconnected(nodeID ids.NodeID) {
	if _, isBeacon := b.beacons.GetValidator(constants.PrimaryNetworkID, nodeID); isBeacon {
		atomic.AddInt64(&b.numConns, -1)
	}
	b.Router.Disconnected(nodeID)
}
