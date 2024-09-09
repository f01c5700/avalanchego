// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"go.uber.org/zap"

	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/snow/networking/router"
	"github.com/f01c5700/avalanchego/snow/validators"
	"github.com/f01c5700/avalanchego/utils/constants"
	"github.com/f01c5700/avalanchego/utils/logging"
	"github.com/f01c5700/avalanchego/version"
)

type insecureValidatorManager struct {
	router.Router
	log    logging.Logger
	vdrs   validators.Manager
	weight uint64
}

func (i *insecureValidatorManager) Connected(vdrID ids.NodeID, nodeVersion *version.Application, subnetID ids.ID) {
	if constants.PrimaryNetworkID == subnetID {
		// Sybil protection is disabled so we don't have a txID that added the
		// peer as a validator. Because each validator needs a txID associated
		// with it, we hack one together by padding the nodeID with zeroes.
		dummyTxID := ids.Empty
		copy(dummyTxID[:], vdrID.Bytes())

		err := i.vdrs.AddStaker(constants.PrimaryNetworkID, vdrID, nil, dummyTxID, i.weight)
		if err != nil {
			i.log.Error("failed to add validator",
				zap.Stringer("nodeID", vdrID),
				zap.Stringer("subnetID", constants.PrimaryNetworkID),
				zap.Error(err),
			)
		}
	}
	i.Router.Connected(vdrID, nodeVersion, subnetID)
}

func (i *insecureValidatorManager) Disconnected(vdrID ids.NodeID) {
	// RemoveWeight will only error here if there was an error reported during
	// Add.
	err := i.vdrs.RemoveWeight(constants.PrimaryNetworkID, vdrID, i.weight)
	if err != nil {
		i.log.Error("failed to remove weight",
			zap.Stringer("nodeID", vdrID),
			zap.Stringer("subnetID", constants.PrimaryNetworkID),
			zap.Error(err),
		)
	}
	i.Router.Disconnected(vdrID)
}
