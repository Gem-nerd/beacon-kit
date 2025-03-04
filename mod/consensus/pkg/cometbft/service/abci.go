// SPDX-License-Identifier: BUSL-1.1
//
// Copyright (C) 2024, Berachain Foundation. All rights reserved.
// Use of this software is governed by the Business Source License included
// in the LICENSE file of this repository and at www.mariadb.com/bsl11.
//
// ANY USE OF THE LICENSED WORK IN VIOLATION OF THIS LICENSE WILL AUTOMATICALLY
// TERMINATE YOUR RIGHTS UNDER THIS LICENSE FOR THE CURRENT AND ALL OTHER
// VERSIONS OF THE LICENSED WORK.
//
// THIS LICENSE DOES NOT GRANT YOU ANY RIGHT IN ANY TRADEMARK OR LOGO OF
// LICENSOR OR ITS AFFILIATES (PROVIDED THAT YOU MAY USE A TRADEMARK OR LOGO OF
// LICENSOR AS EXPRESSLY REQUIRED BY THIS LICENSE).
//
// TO THE EXTENT PERMITTED BY APPLICABLE LAW, THE LICENSED WORK IS PROVIDED ON
// AN “AS IS” BASIS. LICENSOR HEREBY DISCLAIMS ALL WARRANTIES AND CONDITIONS,
// EXPRESS OR IMPLIED, INCLUDING (WITHOUT LIMITATION) WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, NON-INFRINGEMENT, AND
// TITLE.
//
//nolint:contextcheck // its fine.
package cometbft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"cosmossdk.io/store/rootmulti"
	ctypes "github.com/berachain/beacon-kit/mod/consensus-types/pkg/types"
	"github.com/berachain/beacon-kit/mod/consensus/pkg/types"
	errorsmod "github.com/berachain/beacon-kit/mod/errors"
	"github.com/berachain/beacon-kit/mod/primitives/pkg/encoding/json"
	math "github.com/berachain/beacon-kit/mod/primitives/pkg/math"
	cmtabci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/api/cometbft/types/v1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/sourcegraph/conc/iter"
)

//nolint:gocognit // todo fix.
func (app *Service) InitChain(
	_ context.Context,
	req *cmtabci.InitChainRequest,
) (*cmtabci.InitChainResponse, error) {
	if req.ChainId != app.chainID {
		return nil, fmt.Errorf(
			"invalid chain-id on InitChain; expected: %s, got: %s",
			app.chainID,
			req.ChainId,
		)
	}

	// On a new chain, we consider the init chain block height as 0, even though
	// req.InitialHeight is 1 by default.
	initHeader := cmtproto.Header{ChainID: req.ChainId, Time: req.Time}
	app.logger.Info(
		"InitChain",
		"initialHeight",
		req.InitialHeight,
		"chainID",
		req.ChainId,
	)

	// Set the initial height, which will be used to determine if we are
	// proposing
	// or processing the first block or not.
	app.initialHeight = req.InitialHeight
	if app.initialHeight == 0 { // If initial height is 0, set it to 1
		app.initialHeight = 1
	}

	// if req.InitialHeight is > 1, then we set the initial version on all
	// stores
	if req.InitialHeight > 1 {
		initHeader.Height = req.InitialHeight
		if err := app.cms.SetInitialVersion(req.InitialHeight); err != nil {
			return nil, err
		}
	}

	app.setState(execModeFinalize)

	defer func() {
		// InitChain represents the state of the application BEFORE the first
		// block, i.e. the genesis block. This means that when processing the
		// app's InitChain handler, the block height is zero by default.
		// However, after Commit is called
		// the height needs to reflect the true block height.
		initHeader.Height = req.InitialHeight
		app.finalizeBlockState.SetContext(
			app.finalizeBlockState.Context().WithBlockHeader(initHeader),
		)
	}()

	if app.finalizeBlockState == nil {
		return nil, errors.New("finalizeBlockState is nil")
	}

	// add block gas meter for any genesis transactions (allow infinite gas)
	app.finalizeBlockState.SetContext(
		app.finalizeBlockState.Context(),
	)

	res, err := app.initChainer(app.finalizeBlockState.Context(), req)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return nil, errors.New(
			"application init chain handler returned a nil response",
		)
	}

	if len(req.Validators) > 0 {
		if len(req.Validators) != len(res.Validators) {
			return nil, fmt.Errorf(
				"len(RequestInitChain.Validators) != len(GenesisValidators) (%d != %d)",
				len(req.Validators),
				len(res.Validators),
			)
		}

		sort.Sort(cmtabci.ValidatorUpdates(req.Validators))

		for i := range res.Validators {
			if req.Validators[i].Power != res.Validators[i].Power {
				return nil, errors.New("mismatched power")
			}
			if !bytes.Equal(
				req.Validators[i].PubKeyBytes, res.Validators[i].
					PubKeyBytes) {
				return nil, errors.New("mismatched pubkey bytes")
			}

			if req.
				Validators[i].PubKeyType != res.
				Validators[i].PubKeyType {
				return nil, errors.New("mismatched pubkey types")
			}
		}
	}

	// NOTE: We don't commit, but FinalizeBlock for block InitialHeight starts
	// from
	// this FinalizeBlockState.
	return &cmtabci.InitChainResponse{
		ConsensusParams: res.ConsensusParams,
		Validators:      res.Validators,
		AppHash:         app.LastCommitID().Hash,
	}, nil
}

// InitChainer initializes the chain.
func (app *Service) initChainer(
	ctx sdk.Context,
	req *cmtabci.InitChainRequest,
) (*cmtabci.InitChainResponse, error) {
	var genesisState map[string]json.RawMessage
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		return nil, err
	}
	valUpdates, err := app.Middleware.InitGenesis(
		ctx,
		[]byte(genesisState["beacon"]),
	)
	if err != nil {
		return nil, err
	}

	convertedValUpdates, err := iter.MapErr(
		valUpdates,
		convertValidatorUpdate[cmtabci.ValidatorUpdate],
	)
	if err != nil {
		return nil, err
	}

	return &cmtabci.InitChainResponse{
		Validators: convertedValUpdates,
	}, nil
}

func (app *Service) Info(
	context.Context,
	*cmtabci.InfoRequest,
) (*cmtabci.InfoResponse, error) {
	lastCommitID := app.cms.LastCommitID()
	appVersion := InitialAppVersion
	if lastCommitID.Version > 0 {
		ctx, err := app.CreateQueryContext(lastCommitID.Version, false)
		if err != nil {
			return nil, fmt.Errorf("failed creating query context: %w", err)
		}
		appVersion, err = app.AppVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed getting app version: %w", err)
		}
	}

	return &cmtabci.InfoResponse{
		Data:             app.name,
		Version:          app.version,
		AppVersion:       appVersion,
		LastBlockHeight:  lastCommitID.Version,
		LastBlockAppHash: lastCommitID.Hash,
	}, nil
}

// PrepareProposal implements the PrepareProposal ABCI method and returns a
// ResponsePrepareProposal object to the client.
func (app *Service) PrepareProposal(
	_ context.Context,
	req *cmtabci.PrepareProposalRequest,
) (*cmtabci.PrepareProposalResponse, error) {
	app.setState(execModePrepareProposal)

	// CometBFT must never call PrepareProposal with a height of 0.
	if req.Height < 1 {
		return nil, errors.New("PrepareProposal called with invalid height")
	}

	app.prepareProposalState.SetContext(
		app.getContextForProposal(
			app.prepareProposalState.Context(),
			req.Height,
		),
	)

	app.prepareProposalState.SetContext(app.prepareProposalState.Context())

	blkBz, sidecarsBz, err := app.Middleware.PrepareProposal(
		app.prepareProposalState.Context(), &types.SlotData[
			*ctypes.AttestationData,
			*ctypes.SlashingInfo,
		]{
			Slot: math.Slot(req.Height),
		},
	)
	if err != nil {
		app.logger.Error(
			"failed to prepare proposal",
			"height",
			req.Height,
			"time",
			req.Time,
			"err",
			err,
		)
		return &cmtabci.PrepareProposalResponse{Txs: req.Txs}, nil
	}

	return &cmtabci.PrepareProposalResponse{
		Txs: [][]byte{blkBz, sidecarsBz},
	}, nil
}

// ProcessProposal implements the ProcessProposal ABCI method and returns a
// ResponseProcessProposal object to the client.
func (app *Service) ProcessProposal(
	_ context.Context,
	req *cmtabci.ProcessProposalRequest,
) (*cmtabci.ProcessProposalResponse, error) {
	// CometBFT must never call ProcessProposal with a height of 0.
	if req.Height < 1 {
		return nil, errors.New("ProcessProposal called with invalid height")
	}

	app.setState(execModeProcessProposal)

	// Since the application can get access to FinalizeBlock state and write to
	// it, we must be sure to reset it in case ProcessProposal timeouts and is
	// called
	// again in a subsequent round. However, we only want to do this after we've
	// processed the first block, as we want to avoid overwriting the
	// finalizeState
	// after state changes during InitChain.
	if req.Height > app.initialHeight {
		app.setState(execModeFinalize)
	}

	app.processProposalState.SetContext(
		app.getContextForProposal(
			app.processProposalState.Context(),
			req.Height,
		),
	)

	resp, err := app.Middleware.ProcessProposal(
		app.processProposalState.Context(),
		req,
	)
	if err != nil {
		app.logger.Error(
			"failed to process proposal",
			"height",
			req.Height,
			"time",
			req.Time,
			"hash",
			fmt.Sprintf("%X", req.Hash),
			"err",
			err,
		)
		return &cmtabci.ProcessProposalResponse{
			Status: cmtabci.PROCESS_PROPOSAL_STATUS_REJECT,
		}, nil
	}

	return resp, nil
}

func (app *Service) internalFinalizeBlock(
	ctx context.Context,
	req *cmtabci.FinalizeBlockRequest,
) (*cmtabci.FinalizeBlockResponse, error) {
	if err := app.validateFinalizeBlockHeight(req); err != nil {
		return nil, err
	}

	if app.finalizeBlockState == nil {
		app.setState(execModeFinalize)
	}
	if app.finalizeBlockState == nil {
		return nil, errors.New("finalizeBlockState is nil")
	}
	app.finalizeBlockState.SetContext(app.finalizeBlockState.Context())

	// First check for an abort signal after beginBlock, as it's the first place
	// we spend any significant amount of time.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// continue
	}

	app.finalizeBlockState.SetContext(
		app.finalizeBlockState.Context(),
	)

	// Iterate over all raw transactions in the proposal and attempt to execute
	// them, gathering the execution results.
	//
	// NOTE: Not all raw transactions may adhere to the sdk.Tx interface, e.g.
	// vote extensions, so skip those.
	txResults := make([]*cmtabci.ExecTxResult, 0, len(req.Txs))
	for range req.Txs {
		// check after every tx if we should abort
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// continue
		}

		//nolint:mnd // its okay for now.
		txResults = append(txResults, &cmtabci.ExecTxResult{
			Codespace: "sdk",
			Code:      2,
			Log:       "skip decoding",
			GasWanted: 0,
			GasUsed:   0,
		})
	}

	finalizeBlock, err := app.Middleware.FinalizeBlock(
		app.finalizeBlockState.Context(),
		req,
	)
	if err != nil {
		return nil, err
	}

	valUpdates, err := iter.MapErr(
		finalizeBlock,
		convertValidatorUpdate[cmtabci.ValidatorUpdate],
	)
	if err != nil {
		return nil, err
	}

	// check after finalizeBlock if we should abort, to avoid propagating the
	// result
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// continue
	}

	cp := app.GetConsensusParams(app.finalizeBlockState.Context())
	return &cmtabci.FinalizeBlockResponse{
		TxResults:             txResults,
		ValidatorUpdates:      valUpdates,
		ConsensusParamUpdates: &cp,
	}, nil
}

func (app *Service) FinalizeBlock(
	_ context.Context,
	req *cmtabci.FinalizeBlockRequest,
) (*cmtabci.FinalizeBlockResponse, error) {
	res, err := app.internalFinalizeBlock(context.Background(), req)
	if res != nil {
		res.AppHash = app.workingHash()
	}

	return res, err
}

// Commit implements the ABCI interface. It will commit all state that exists in
// the deliver state's multi-store and includes the resulting commit ID in the
// returned cmtabci.ResponseCommit. Commit will set the check state based on the
// latest header and reset the deliver state. Also, if a non-zero halt height is
// defined in config, Commit will execute a deferred function call to check
// against that height and gracefully halt if it matches the latest committed
// height.
func (app *Service) Commit(
	context.Context, *cmtabci.CommitRequest,
) (*cmtabci.CommitResponse, error) {
	if app.finalizeBlockState == nil {
		return nil, errors.New("finalizeBlockState is nil")
	}
	header := app.finalizeBlockState.Context().BlockHeader()
	retainHeight := app.GetBlockRetentionHeight(header.Height)

	rms, ok := app.cms.(*rootmulti.Store)
	if ok {
		rms.SetCommitHeader(header)
	}

	app.cms.Commit()

	resp := &cmtabci.CommitResponse{
		RetainHeight: retainHeight,
	}

	app.finalizeBlockState = nil

	return resp, nil
}

// workingHash gets the apphash that will be finalized in commit.
// These writes will be persisted to the root multi-store (app.cms) and flushed
// to disk in the Commit phase. This means when the ABCI client requests
// Commit(), the application state transitions will be flushed to disk and as a
// result, but we already have
// an application Merkle root.
func (app *Service) workingHash() []byte {
	// Write the FinalizeBlock state into branched storage and commit the
	// MultiStore. The write to the FinalizeBlock state writes all state
	// transitions to the root
	// MultiStore (app.cms) so when Commit() is called it persists those values.
	if app.finalizeBlockState == nil {
		panic("workingHash() called before FinalizeBlock()")
	}
	app.finalizeBlockState.ms.Write()

	// Get the hash of all writes in order to return the apphash to the comet in
	// finalizeBlock.
	commitHash := app.cms.WorkingHash()
	app.logger.Debug(
		"hash of all writes",
		"workingHash",
		fmt.Sprintf("%X", commitHash),
	)

	return commitHash
}

// getContextForProposal returns the correct Context for PrepareProposal and
// ProcessProposal. We use finalizeBlockState on the first block to be able to
// access any state changes made in InitChain.
func (app *Service) getContextForProposal(
	ctx sdk.Context,
	height int64,
) sdk.Context {
	if height == app.initialHeight {
		if app.finalizeBlockState == nil {
			return ctx
		}
		ctx, _ = app.finalizeBlockState.Context().CacheContext()
		return ctx
	}

	return ctx
}

// CreateQueryContext creates a new sdk.Context for a query, taking as args
// the block height and whether the query needs a proof or not.
func (app *Service) CreateQueryContext(
	height int64,
	prove bool,
) (sdk.Context, error) {
	// use custom query multi-store if provided
	lastBlockHeight := app.cms.LatestVersion()
	if lastBlockHeight == 0 {
		return sdk.Context{}, errorsmod.Wrapf(
			sdkerrors.ErrInvalidHeight,
			"%s is not ready; please wait for first block",
			app.name,
		)
	}

	if height > lastBlockHeight {
		return sdk.Context{},
			errorsmod.Wrap(
				sdkerrors.ErrInvalidHeight,
				"cannot query with height in the future; please provide a valid height",
			)
	}

	// when a client did not provide a query height, manually inject the latest
	if height == 0 {
		height = lastBlockHeight
	}

	if height <= 1 && prove {
		return sdk.Context{},
			errorsmod.Wrap(
				sdkerrors.ErrInvalidRequest,
				"cannot query with proof when height <= 1; please provide a valid height",
			)
	}

	cacheMS, err := app.cms.CacheMultiStoreWithVersion(height)
	if err != nil {
		return sdk.Context{},
			errorsmod.Wrapf(
				sdkerrors.ErrNotFound,
				"failed to load state at height %d; %s (latest height: %d)",
				height,
				err,
				lastBlockHeight,
			)
	}

	return sdk.NewContext(cacheMS, true, app.logger), nil
}

// GetBlockRetentionHeight returns the height for which all blocks below this
// height
// are pruned from CometBFT. Given a commitment height and a non-zero local
// minRetainBlocks configuration, the retentionHeight is the smallest height
// that
// satisfies:
//
// - Unbonding (safety threshold) time: The block interval in which validators
// can be economically punished for misbehavior. Blocks in this interval must be
// auditable e.g. by the light client.
//
// - Logical store snapshot interval: The block interval at which the underlying
// logical store database is persisted to disk, e.g. every 10000 heights. Blocks
// since the last IAVL snapshot must be available for replay on application
// restart.
//
// - State sync snapshots: Blocks since the oldest available snapshot must be
// available for state sync nodes to catch up (oldest because a node may be
// restoring an old snapshot while a new snapshot was taken).
//
// - Local (minRetainBlocks) config: Archive nodes may want to retain more or
// all blocks, e.g. via a local config option min-retain-blocks. There may also
// be a need to vary retention for other nodes, e.g. sentry nodes which do not
// need historical blocks.
func (app *Service) GetBlockRetentionHeight(commitHeight int64) int64 {
	// pruning is disabled if minRetainBlocks is zero
	if app.minRetainBlocks == 0 {
		return 0
	}

	minNonZero := func(x, y int64) int64 {
		switch {
		case x == 0:
			return y

		case y == 0:
			return x

		case x < y:
			return x

		default:
			return y
		}
	}

	// Define retentionHeight as the minimum value that satisfies all non-zero
	// constraints. All blocks below (commitHeight-retentionHeight) are pruned
	// from CometBFT.
	var retentionHeight int64

	// Define the number of blocks needed to protect against misbehaving
	// validators
	// which allows light clients to operate safely. Note, we piggy back of the
	// evidence parameters instead of computing an estimated number of blocks
	// based
	// on the unbonding period and block commitment time as the two should be
	// equivalent.
	if app.finalizeBlockState == nil {
		return 0
	}
	cp := app.GetConsensusParams(app.finalizeBlockState.Context())
	if cp.Evidence != nil && cp.Evidence.MaxAgeNumBlocks > 0 {
		retentionHeight = commitHeight - cp.Evidence.MaxAgeNumBlocks
	}

	//#nosec:G701 // bet.
	v := commitHeight - int64(app.minRetainBlocks)
	retentionHeight = minNonZero(retentionHeight, v)

	if retentionHeight <= 0 {
		// prune nothing in the case of a non-positive height
		return 0
	}

	return retentionHeight
}
