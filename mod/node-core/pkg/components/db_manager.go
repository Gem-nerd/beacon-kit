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

package components

import (
	"cosmossdk.io/depinject"
	"github.com/berachain/beacon-kit/mod/log"
	"github.com/berachain/beacon-kit/mod/storage/pkg/manager"
	"github.com/berachain/beacon-kit/mod/storage/pkg/pruner"
)

// DBManagerInput is the input for the dep inject framework.
type DBManagerInput[
	AvailabilityStoreT pruner.Prunable,
	DepositStoreT pruner.Prunable,
	LoggerT any,
] struct {
	depinject.In
	AvailabilityPruner pruner.Pruner[AvailabilityStoreT]
	DepositPruner      pruner.Pruner[DepositStoreT]
	Logger             LoggerT
}

// ProvideDBManager provides a DBManager for the depinject framework.
func ProvideDBManager[
	AvailabilityStoreT pruner.Prunable,
	DepositStoreT pruner.Prunable,
	LoggerT log.AdvancedLogger[any, LoggerT],
](
	in DBManagerInput[AvailabilityStoreT, DepositStoreT, LoggerT],
) (*manager.DBManager, error) {
	return manager.NewDBManager(
		in.Logger.With("service", "db-manager"),
		in.DepositPruner,
		in.AvailabilityPruner,
	)
}
