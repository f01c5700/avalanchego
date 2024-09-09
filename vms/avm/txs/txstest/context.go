// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txstest

import (
	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/snow"
	"github.com/f01c5700/avalanchego/vms/avm/config"
	"github.com/f01c5700/avalanchego/wallet/chain/x/builder"
)

func newContext(
	ctx *snow.Context,
	cfg *config.Config,
	feeAssetID ids.ID,
) *builder.Context {
	return &builder.Context{
		NetworkID:        ctx.NetworkID,
		BlockchainID:     ctx.XChainID,
		AVAXAssetID:      feeAssetID,
		BaseTxFee:        cfg.TxFee,
		CreateAssetTxFee: cfg.CreateAssetTxFee,
	}
}
