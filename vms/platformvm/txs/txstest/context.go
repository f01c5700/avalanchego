// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txstest

import (
	"github.com/f01c5700/avalanchego/snow"
	"github.com/f01c5700/avalanchego/vms/components/gas"
	"github.com/f01c5700/avalanchego/vms/platformvm/config"
	"github.com/f01c5700/avalanchego/vms/platformvm/state"
	"github.com/f01c5700/avalanchego/wallet/chain/p/builder"
)

func newContext(
	ctx *snow.Context,
	config *config.Config,
	state state.State,
) *builder.Context {
	var (
		timestamp      = state.GetTimestamp()
		builderContext = &builder.Context{
			NetworkID:   ctx.NetworkID,
			AVAXAssetID: ctx.AVAXAssetID,
		}
	)
	switch {
	case config.UpgradeConfig.IsEtnaActivated(timestamp):
		builderContext.ComplexityWeights = config.DynamicFeeConfig.Weights
		builderContext.GasPrice = gas.CalculatePrice(
			config.DynamicFeeConfig.MinPrice,
			state.GetFeeState().Excess,
			config.DynamicFeeConfig.ExcessConversionConstant,
		)
	case config.UpgradeConfig.IsApricotPhase3Activated(timestamp):
		builderContext.StaticFeeConfig = config.StaticFeeConfig
	default:
		builderContext.StaticFeeConfig = config.StaticFeeConfig
		builderContext.StaticFeeConfig.CreateSubnetTxFee = config.CreateAssetTxFee
		builderContext.StaticFeeConfig.CreateBlockchainTxFee = config.CreateAssetTxFee
	}
	return builderContext
}
