// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"github.com/f01c5700/avalanchego/snow"
	"github.com/f01c5700/avalanchego/snow/uptime"
	"github.com/f01c5700/avalanchego/utils"
	"github.com/f01c5700/avalanchego/utils/timer/mockable"
	"github.com/f01c5700/avalanchego/vms/platformvm/config"
	"github.com/f01c5700/avalanchego/vms/platformvm/fx"
	"github.com/f01c5700/avalanchego/vms/platformvm/reward"
	"github.com/f01c5700/avalanchego/vms/platformvm/utxo"
)

type Backend struct {
	Config       *config.Config
	Ctx          *snow.Context
	Clk          *mockable.Clock
	Fx           fx.Fx
	FlowChecker  utxo.Verifier
	Uptimes      uptime.Calculator
	Rewards      reward.Calculator
	Bootstrapped *utils.Atomic[bool]
}
