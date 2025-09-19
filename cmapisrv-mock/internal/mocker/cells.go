// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"errors"

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
	"github.com/cilium/cilium/pkg/lock"
)

var Cell = cell.Module(
	"mocker",
	"Cilium Cluster Mesh Mocker",

	cell.Config(option.DefaultLegacyClusterMeshConfig),

	cell.Config(defaultConfig),
	cell.Invoke(config.validate),
	cell.Config(defaultRndcfg),

	controller.Cell,

	kvstore.Cell(kvstore.EtcdBackendName),
	cell.Invoke(func(client kvstore.Client) error {
		if !client.IsEnabled() {
			return errors.New("KVStore client not configured, cannot continue")
		}

		return nil
	}),

	heartbeat.Enabled,
	heartbeat.Cell,
	cell.Provide(func() (syncstate.SyncState, kvstore.ExtraOptions) {
		ss := syncstate.SyncState{StoppableWaitGroup: lock.NewStoppableWaitGroup()}
		return ss, kvstore.ExtraOptions{
			BootstrapComplete: ss.WaitChannel(),
		}
	}),
	store.Cell,

	cmhealth.HealthAPIServerCell,
	cell.Provide(func() types.ClusterInfo { return types.DefaultClusterInfo }),
	cell.Provide((*mocker).HealthEndpoints),

	gops.Cell(defaults.EnableGops, defaults.GopsPortKVStoreMesh),
	cmmetrics.Cell,

	cell.Provide(newRandom),
	cell.Provide(newMocker),
	cell.Invoke(func(_ *mocker) {}),
)
