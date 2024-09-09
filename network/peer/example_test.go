// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/f01c5700/avalanchego/message"
	"github.com/f01c5700/avalanchego/snow/networking/router"
	"github.com/f01c5700/avalanchego/utils/constants"
)

func ExampleStartTestPeer() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	peerIP := netip.AddrPortFrom(
		netip.IPv6Loopback(),
		9651,
	)
	peer, err := StartTestPeer(
		ctx,
		peerIP,
		constants.LocalID,
		router.InboundHandlerFunc(func(_ context.Context, msg message.InboundMessage) {
			fmt.Printf("handling %s\n", msg.Op())
		}),
	)
	if err != nil {
		panic(err)
	}

	// Send messages here with [peer.Send].

	peer.StartClose()
	err = peer.AwaitClosed(ctx)
	if err != nil {
		panic(err)
	}
}
