// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	cmhealth "github.com/cilium/cilium/clustermesh-apiserver/health"
	cmmetrics "github.com/cilium/cilium/clustermesh-apiserver/metrics"
	"github.com/cilium/cilium/clustermesh-apiserver/syncstate"
	"github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/controller"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/gops"
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/hive/job"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/heartbeat"
	"github.com/cilium/cilium/pkg/kvstore/store"
)

var Cell = cell.Module(
	"mocker",
	"Cilium Cluster Mesh Mocker",

	cell.Config(defaultConfig),
	cell.Config(defaultRndcfg),

	controller.Cell,
	job.Cell,

	kvstore.Cell(kvstore.EtcdBackendName),
	heartbeat.Cell,
	cell.Provide(func() *kvstore.ExtraOptions { return nil }),
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
