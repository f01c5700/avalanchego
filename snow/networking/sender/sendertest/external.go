// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sendertest

import (
	"errors"
	"testing"

	"github.com/f01c5700/avalanchego/ids"
	"github.com/f01c5700/avalanchego/message"
	"github.com/f01c5700/avalanchego/snow/engine/common"
	"github.com/f01c5700/avalanchego/snow/networking/sender"
	"github.com/f01c5700/avalanchego/subnets"
	"github.com/f01c5700/avalanchego/utils/set"
)

var (
	_ sender.ExternalSender = (*External)(nil)

	errSend = errors.New("unexpectedly called Send")
)

// External is a test sender
type External struct {
	TB testing.TB

	CantSend bool

	SendF func(msg message.OutboundMessage, config common.SendConfig, subnetID ids.ID, allower subnets.Allower) set.Set[ids.NodeID]
}

// Default set the default callable value to [cant]
func (s *External) Default(cant bool) {
	s.CantSend = cant
}

func (s *External) Send(
	msg message.OutboundMessage,
	config common.SendConfig,
	subnetID ids.ID,
	allower subnets.Allower,
) set.Set[ids.NodeID] {
	if s.SendF != nil {
		return s.SendF(msg, config, subnetID, allower)
	}
	if s.CantSend {
		if s.TB != nil {
			s.TB.Helper()
			s.TB.Fatal(errSend)
		}
	}
	return nil
}
