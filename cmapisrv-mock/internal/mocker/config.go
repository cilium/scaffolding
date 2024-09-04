// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"fmt"

	"github.com/spf13/pflag"

	wgTypes "github.com/cilium/cilium/pkg/wireguard/types"
)

type encryptionMode string

func (em encryptionMode) toKey() uint8 {
	switch em {
	case encryptionModeIPSec:
		return 1
	case encryptionModeWireGuard:
		return wgTypes.StaticEncryptKey
	default:
		return 0
	}
}

const (
	encryptionModeDisabled  = encryptionMode("disabled")
	encryptionModeIPSec     = encryptionMode("ipsec")
	encryptionModeWireGuard = encryptionMode("wireguard")
)

type config struct {
	EnableIPv6 bool
	Encryption encryptionMode

	Clusters       uint
	FirstClusterID uint

	Nodes           uint
	NodesQPS        float64
	NodeAnnotations map[string]string

	Identities    uint
	IdentitiesQPS float64

	Endpoints    uint
	EndpointsQPS float64

	Services    uint
	ServicesQPS float64
}

var defaultConfig = config{
	EnableIPv6: false,
	Encryption: encryptionModeDisabled,

	Clusters:       1,
	FirstClusterID: 1,

	Nodes:      10,
	Identities: 10,
	Endpoints:  10,
	Services:   10,
}

func (def config) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-ipv6", def.EnableIPv6, "Enable IPv6")
	flags.String("encryption", string(def.Encryption), "Cilium's encryption mode; supported values: disabled|ipsec|wireguard")

	flags.Uint("clusters", def.Clusters, "Number of clusters to mock")
	flags.Uint("first-cluster-id", def.FirstClusterID, "Cluster ID of the initial cluster")

	flags.Uint("nodes", def.Nodes, "Number of nodes to mock (per cluster)")
	flags.Float64("nodes-qps", def.NodesQPS, "Node QPS (per cluster)")
	flags.StringToString("node-annotations", def.NodeAnnotations, "Extra annotations configured for each mocked node")

	flags.Uint("identities", def.Identities, "Number of identities to mock (per cluster)")
	flags.Float64("identities-qps", def.IdentitiesQPS, "Identities QPS (per cluster)")

	flags.Uint("endpoints", def.Endpoints, "Number of endpoints to mock (per cluster)")
	flags.Float64("endpoints-qps", def.EndpointsQPS, "Endpoints QPS (per cluster)")

	flags.Uint("services", def.Endpoints, "Number of services to mock (per cluster)")
	flags.Float64("services-qps", def.EndpointsQPS, "Services QPS (per cluster)")
}

func (cfg config) validate() error {
	switch cfg.Encryption {
	case encryptionModeDisabled, encryptionModeIPSec, encryptionModeWireGuard:
	default:
		return fmt.Errorf("unsupported encryption mode %q; must be one of disabled|ipsec|wireguard", cfg.Encryption)
	}

	return nil
}

type rndcfg struct {
	RandomNodeIP4 string
	RandomNodeIP6 string
	RandomPodIP4  string
	RandomPodIP6  string
	RandomSvcIP4  string
	RandomSvcIP6  string
}

var defaultRndcfg = rndcfg{
	RandomNodeIP4: "172.16.0.0",
	RandomNodeIP6: "fc00::0",
	RandomPodIP4:  "10.0.0.0",
	RandomPodIP6:  "fd00::0",
	RandomSvcIP4:  "172.252.0.0",
	RandomSvcIP6:  "fdff::0",
}

func (def rndcfg) Flags(flags *pflag.FlagSet) {
	flags.String("random-node-ip4", def.RandomNodeIP4, "The first mocked node IPv4 address")
	flags.String("random-node-ip6", def.RandomNodeIP6, "The first mocked node IPv6 address")

	flags.String("random-pod-ip4", def.RandomPodIP4, "The first mocked pod IPv4 address")
	flags.String("random-pod-ip6", def.RandomPodIP6, "The first mocked pod IPv6 address")

	flags.String("random-svc-ip4", def.RandomSvcIP4, "The first mocked service IPv4 address")
	flags.String("random-svc-ip6", def.RandomSvcIP6, "The first mocked service IPv6 address")
}
