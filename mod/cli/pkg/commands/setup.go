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

package commands

import (
	"github.com/berachain/beacon-kit/mod/cli/pkg/commands/deposit"
	"github.com/berachain/beacon-kit/mod/cli/pkg/commands/genesis"
	"github.com/berachain/beacon-kit/mod/cli/pkg/commands/jwt"
	"github.com/berachain/beacon-kit/mod/cli/pkg/commands/server"
	servertypes "github.com/berachain/beacon-kit/mod/cli/pkg/commands/server/types"
	"github.com/berachain/beacon-kit/mod/cli/pkg/flags"
	cmtcli "github.com/berachain/beacon-kit/mod/consensus/pkg/cometbft/cli"
	ccometbft "github.com/berachain/beacon-kit/mod/consensus/pkg/cometbft/service"
	"github.com/berachain/beacon-kit/mod/node-core/pkg/types"
	"github.com/berachain/beacon-kit/mod/primitives/pkg/common"
	"github.com/berachain/beacon-kit/mod/primitives/pkg/constraints"
	"github.com/cosmos/cosmos-sdk/version"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
)

// DefaultRootCommandSetup sets up the default commands for the root command.
func DefaultRootCommandSetup[
	T types.Node,
	ExecutionPayloadT constraints.EngineType[ExecutionPayloadT],
](
	root *Root,
	mm *ccometbft.Service,
	appCreator servertypes.AppCreator[T],
	chainSpec common.ChainSpec,
) {
	// Add all the commands to the root command.
	root.cmd.AddCommand(
		// `comet`
		cmtcli.Commands(appCreator),
		// `init`
		genutilcli.InitCmd(mm),
		// `genesis`
		genesis.Commands(chainSpec),
		// `deposit`
		deposit.Commands[ExecutionPayloadT](chainSpec),
		// `jwt`
		jwt.Commands(),
		// `rollback`
		server.NewRollbackCmd(appCreator),
		// `start`
		server.StartCmdWithOptions(appCreator, server.StartCmdOptions[T]{
			AddFlags: flags.AddBeaconKitFlags,
		}),
		// `status`
		cmtcli.StatusCommand(),
		// `version`
		version.NewVersionCommand(),
	)
}
