// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"github.com/cilium/hive/cell"

	cmhealth "github.com/cilium/cilium/clustermesh-apiserver/health"
	cmmetrics "github.com/cilium/cilium/clustermesh-apiserver/metrics"
	"github.com/cilium/cilium/clustermesh-apiserver/option"
	"github.com/cilium/cilium/clustermesh-apiserver/syncstate"
	"github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/controller"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/gops"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/heartbeat"
	"github.com/cilium/cilium/pkg/kvstore/store"
)

var Cell = cell.Module(
	"mocker",
	"Cilium Cluster Mesh Mocker",

	cell.Config(option.DefaultLegacyClusterMeshConfig),

	cell.Config(defaultConfig),
	cell.Invoke(config.validate),
	cell.Config(defaultRndcfg),

	controller.Cell,

	kvstore.Cell,
	heartbeat.Cell,
	cell.Provide(func(ss syncstate.SyncState) *kvstore.ExtraOptions {
		return &kvstore.ExtraOptions{
			BootstrapComplete: ss.WaitChannel(),
		}
	}),
	store.Cell,

	cmhealth.HealthAPIServerCell,
	cell.Provide(func() types.ClusterInfo { return types.DefaultClusterInfo }),
	syncstate.Cell,
	cell.Provide((*mocker).HealthEndpoints),

	gops.Cell(defaults.GopsPortKVStoreMesh),
	cmmetrics.Cell,

	cell.Provide(newRandom),
	cell.Provide(newMocker),
	cell.Invoke(func(_ *mocker) {}),
)
