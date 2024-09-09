// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/utils/constants"
	"github.com/f01c5700/avalanchego/utils/crypto/bls"
	"github.com/f01c5700/avalanchego/vms/platformvm/warp"
	"github.com/f01c5700/avalanchego/vms/platformvm/warp/signertest"
)

func TestSigner(t *testing.T) {
	for name, test := range signertest.SignerTests {
		t.Run(name, func(t *testing.T) {
			sk, err := bls.NewSecretKey()
			require.NoError(t, err)

			chainID := ids.GenerateTestID()
			s := warp.NewSigner(sk, constants.UnitTestID, chainID)

			test(t, s, sk, constants.UnitTestID, chainID)
		})
	}
}
