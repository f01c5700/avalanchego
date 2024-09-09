// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/snow"
	"github.com/f01c5700/avalanchego/utils/constants"
	"github.com/f01c5700/avalanchego/utils/logging"
	"github.com/f01c5700/avalanchego/vms/components/gas"
	"github.com/f01c5700/avalanchego/vms/platformvm/txs/fee"
)

const Alias = "P"

type Context struct {
	NetworkID         uint32
	AVAXAssetID       ids.ID
	StaticFeeConfig   fee.StaticConfig
	ComplexityWeights gas.Dimensions
	GasPrice          gas.Price
}

func NewSnowContext(networkID uint32, avaxAssetID ids.ID) (*snow.Context, error) {
	lookup := ids.NewAliaser()
	return &snow.Context{
		NetworkID:   networkID,
		SubnetID:    constants.PrimaryNetworkID,
		ChainID:     constants.PlatformChainID,
		AVAXAssetID: avaxAssetID,
		Log:         logging.NoLog{},
		BCLookup:    lookup,
	}, lookup.Alias(constants.PlatformChainID, Alias)
}
