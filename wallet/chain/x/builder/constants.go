// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"github.com/f01c5700/avalanchego/vms/avm/block"
	"github.com/f01c5700/avalanchego/vms/avm/fxs"
	"github.com/f01c5700/avalanchego/vms/nftfx"
	"github.com/f01c5700/avalanchego/vms/propertyfx"
	"github.com/f01c5700/avalanchego/vms/secp256k1fx"
)

const (
	SECP256K1FxIndex = 0
	NFTFxIndex       = 1
	PropertyFxIndex  = 2
)

// Parser to support serialization and deserialization
var Parser block.Parser

func init() {
	var err error
	Parser, err = block.NewParser(
		[]fxs.Fx{
			&secp256k1fx.Fx{},
			&nftfx.Fx{},
			&propertyfx.Fx{},
		},
	)
	if err != nil {
		panic(err)
	}
}
