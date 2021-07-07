package chain

import (
	"context"
	"os"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type P2pAPI struct {
	Publish func(ctx context.Context, b []byte) error
}

func NewP2pLotusAPI(a api.FullNode) (*P2pLotusAPI, error) {
	api := &P2pLotusAPI{}
	api.FullNode = a

	ctx := context.TODO()

	var err error
	if extrAPI, ok := os.LookupEnv("EXTRAPI"); ok {
		api.apiClient, api.cclose, err = NewChainRPC(ctx, extrAPI)
		if err != nil {
			return nil, err
		}
	} else {
		h, sub, err := NewPubSub()
		if err != nil {
			return nil, err
		}
		api.sub = sub
		api.host = h

		api.msgtopic, err = api.sub.Join(build.MessagesTopic(NetName))
		if err != nil {
			return nil, err
		}

		api.blktopic, err = api.sub.Join(build.BlocksTopic(NetName))
		if err != nil {
			return nil, err
		}

		dhtOpts := DhtOpts
		dhtOpts = append(dhtOpts, dht.ProtocolPrefix(build.DhtProtocolName(NetName)))
		r, err := dht.New(ctx, api.host, dhtOpts...)
		if err != nil {
			return nil, err
		}

		api.routed = r

		ps, err := build.BuiltinBootstrap()
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if err := api.host.Connect(ctx, p); err == nil {

			} else {
				log.Errorf("connect peer %v error %v", p, err)
			}
		}
		if len(ps) > 5 {
			ps = ps[:5]
		}
		api.bootPeers = ps

		go func() {
			for {
				if len(api.routed.RoutingTable().ListPeers()) < 12 {
					for _, p := range api.bootPeers {
						err := api.host.Connect(ctx, p)
						if err != nil {
							log.Error(err)
						}
					}
				}
				if v, ok := os.LookupEnv("LOG_LEVEL"); ok {
					if v == "DEBUGROUTE" {
						api.routed.RoutingTable().Print()
					}
				}
				freshPeers := api.routed.RoutingTable().ListPeers()
				if len(freshPeers) > 10 {
					freshPeers = freshPeers[:10]
				}
				for _, pid := range freshPeers {
					err := api.host.Network().ClosePeer(pid)
					if err != nil {
						log.Errorf("host networker connect peer %v error %v", pid, err)
					}
				}
				time.Sleep(10 * time.Second)
			}
		}()
	}

	return api, nil
}

type P2pLotusAPI struct {
	api.FullNode

	host      host.Host
	sub       *pubsub.PubSub
	msgtopic  *pubsub.Topic
	blktopic  *pubsub.Topic
	bootPeers []peer.AddrInfo
	routed    *dht.IpfsDHT

	apiClient *P2pAPI
	cclose    jsonrpc.ClientCloser
}

func (a *P2pLotusAPI) MpoolPush(ctx context.Context, m *types.SignedMessage) (cid.Cid, error) {
	b, err := m.Serialize()
	if err != nil {
		return cid.Undef, err
	}
	if a.apiClient != nil {
		err = a.apiClient.Publish(ctx, b)
	} else {
		err = a.msgtopic.Publish(ctx, b)
	}
	if err != nil {
		return cid.Undef, err
	}
	return m.Cid(), nil
}

func (a *P2pLotusAPI) Publish(ctx context.Context, b []byte) error {
	if a.apiClient != nil {
		return a.apiClient.Publish(ctx, b)
	}
	return a.blktopic.Publish(ctx, b)
}

func (a *P2pLotusAPI) GetHost() host.Host {
	return a.host
}

func (a *P2pLotusAPI) GetRoute() *dht.IpfsDHT {
	return a.routed
}

func (a *P2pLotusAPI) GetMsgTopic() *pubsub.Topic {
	return a.msgtopic
}

func (a *P2pLotusAPI) GetBlkTopic() *pubsub.Topic {
	return a.blktopic
}

func (a *P2pLotusAPI) Close() {
	if a.blktopic != nil {
		a.blktopic.Close()
	}
	if a.msgtopic != nil {
		a.msgtopic.Close()
	}
	if a.routed != nil {
		a.routed.Close()
	}
	if a.host != nil {
		a.host.Close()
	}
	if a.cclose != nil {
		a.cclose()
	}
}

func NewPubSub() (host.Host, *pubsub.PubSub, error) {

	ctx := context.TODO()
	host, err := libp2p.New(
		ctx,
		HostOpts...,
	)
	if err != nil {
		return nil, nil, err
	}
	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, nil, err
	}

	return host, ps, nil
}
