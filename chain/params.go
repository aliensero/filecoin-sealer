package chain

import (
	"time"

	"github.com/filecoin-project/lotus/build"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"gitlab.ns/lotus-worker/util"
)

var NetName = util.NsNetworkName("testnetnet")

var HostOpts = []libp2p.Option{
	libp2p.NoListenAddrs,
	libp2p.ConnectionManager(connmgr.NewConnManager(50, 200, 20*time.Second)),
}

var DhtOpts = []dht.Option{
	dht.Mode(dht.ModeAuto),
	dht.ProtocolPrefix(build.DhtProtocolName(NetName)),
	dht.DisableProviders(),
	dht.DisableValues(),
}
