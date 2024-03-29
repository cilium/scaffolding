// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import "github.com/spf13/pflag"

type config struct {
	EnableIPv6 bool

	Clusters       uint
	FirstClusterID uint

	Nodes    uint
	NodesQPS float64

	Identities    uint
	IdentitiesQPS float64

	Endpoints    uint
	EndpointsQPS float64

	Services    uint
	ServicesQPS float64
}

var defaultConfig = config{
	EnableIPv6: false,

	Clusters:       1,
	FirstClusterID: 1,

	Nodes:      10,
	Identities: 10,
	Endpoints:  10,
	Services:   10,
}

func (def config) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-ipv6", def.EnableIPv6, "Enable IPv6")

	flags.Uint("clusters", def.Clusters, "Number of clusters to mock")
	flags.Uint("first-cluster-id", def.FirstClusterID, "Cluster ID of the initial cluster")

	flags.Uint("nodes", def.Nodes, "Number of nodes to mock (per cluster)")
	flags.Float64("nodes-qps", def.NodesQPS, "Node QPS (per cluster)")

	flags.Uint("identities", def.Identities, "Number of identities to mock (per cluster)")
	flags.Float64("identities-qps", def.IdentitiesQPS, "Identities QPS (per cluster)")

	flags.Uint("endpoints", def.Endpoints, "Number of endpoints to mock (per cluster)")
	flags.Float64("endpoints-qps", def.EndpointsQPS, "Endpoints QPS (per cluster)")

	flags.Uint("services", def.Endpoints, "Number of services to mock (per cluster)")
	flags.Float64("services-qps", def.EndpointsQPS, "Services QPS (per cluster)")
}
