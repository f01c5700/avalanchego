// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package propertyfx

import (
	"github.com/f01c5700/avalanchego/vms/components/verify"
	"github.com/f01c5700/avalanchego/vms/secp256k1fx"
)

var _ verify.State = (*MintOutput)(nil)

type MintOutput struct {
	verify.IsState `json:"-"`

	secp256k1fx.OutputOwners `serialize:"true"`
}
