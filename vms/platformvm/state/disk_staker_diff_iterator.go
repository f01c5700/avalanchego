// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
)

const (
	// startDiffKey = [subnetID] + [inverseHeight]
	startDiffKeyLength = ids.IDLen + database.Uint64Size
	// diffKeyNodeIDOffset = [subnetIDLen] + [inverseHeightLen]
	diffKeyNodeIDOffset = ids.IDLen + database.Uint64Size

	// weightValue = [isNegative] + [weight]
	weightValueLength = database.BoolSize + database.Uint64Size
)

var (
	errUnexpectedZeroLenghtNodeID  = errors.New("nodeID has zero length")
	errUnexpectedWeightValueLength = fmt.Errorf("expected weight value length %d", weightValueLength)
)

// marshalStartDiffKey is used to determine the starting key when iterating.
//
// Invariant: the result is a prefix of [marshalDiffKey] when called with the
// same arguments.
func marshalStartDiffKey(subnetID ids.ID, height uint64) []byte {
	key := make([]byte, startDiffKeyLength)
	copy(key, subnetID.Bytes())
	packIterableHeight(key[ids.IDLen:], height)
	return key
}

func marshalDiffKey(subnetID ids.ID, height uint64, nodeID ids.NodeID) []byte {
	key := make([]byte, startDiffKeyLength+len(nodeID))
	copy(key, subnetID.Bytes())
	packIterableHeight(key[ids.IDLen:], height)
	copy(key[diffKeyNodeIDOffset:], nodeID.Bytes())
	return key
}

func unmarshalDiffKey(key []byte) (ids.ID, uint64, ids.NodeID, error) {
	if len(key) < startDiffKeyLength {
		return ids.Empty, 0, ids.EmptyNodeID, errUnexpectedZeroLenghtNodeID
	}
	var (
		subnetID ids.ID
		nodeID   ids.NodeID
	)
	copy(ids.Writable(&subnetID), key)
	height := unpackIterableHeight(key[ids.IDLen:])
	nodeID = ids.NodeIDFromBytes(key[diffKeyNodeIDOffset:])
	return subnetID, height, nodeID, nil
}

func marshalWeightDiff(diff *ValidatorWeightDiff) []byte {
	value := make([]byte, weightValueLength)
	if diff.Decrease {
		value[0] = database.BoolTrue
	}
	binary.BigEndian.PutUint64(value[database.BoolSize:], diff.Amount)
	return value
}

func unmarshalWeightDiff(value []byte) (*ValidatorWeightDiff, error) {
	if len(value) != weightValueLength {
		return nil, errUnexpectedWeightValueLength
	}
	return &ValidatorWeightDiff{
		Decrease: value[0] == database.BoolTrue,
		Amount:   binary.BigEndian.Uint64(value[database.BoolSize:]),
	}, nil
}

// Note: [height] is encoded as a bit flipped big endian number so that
// iterating lexicographically results in iterating in decreasing heights.
//
// Invariant: [key] has sufficient length
func packIterableHeight(key []byte, height uint64) {
	binary.BigEndian.PutUint64(key, ^height)
}

// Because we bit flip the height when constructing the key, we must remember to
// bip flip again here.
//
// Invariant: [key] has sufficient length
func unpackIterableHeight(key []byte) uint64 {
	return ^binary.BigEndian.Uint64(key)
}
