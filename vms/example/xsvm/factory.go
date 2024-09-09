// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package xsvm

import (
	"github.com/f01c5700/avalanchego/utils/logging"
	"github.com/f01c5700/avalanchego/vms"
)

var _ vms.Factory = (*Factory)(nil)

type Factory struct{}

func (*Factory) New(logging.Logger) (interface{}, error) {
	return &VM{}, nil
}
