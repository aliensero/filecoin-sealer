package p2p

import (
	"context"
	"reflect"
	"time"

	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"
	"golang.org/x/xerrors"

	_ "github.com/filecoin-project/lotus/lib/sigs/bls"
	_ "github.com/filecoin-project/lotus/lib/sigs/secp"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/filecoin-project/lotus/node/modules/lp2p"
)

type P2PHostIn struct {
	fx.In

	Opts [][]libp2p.Option `group:"libp2p"`
}

type invoke int

type special struct{ id int }

func as(in interface{}, as interface{}) interface{} {
	outType := reflect.TypeOf(as)

	if outType.Kind() != reflect.Ptr {
		panic("outType is not a pointer")
	}

	if reflect.TypeOf(in).Kind() != reflect.Func {
		ctype := reflect.FuncOf(nil, []reflect.Type{outType.Elem()}, false)

		return reflect.MakeFunc(ctype, func(args []reflect.Value) (results []reflect.Value) {
			out := reflect.New(outType.Elem())
			out.Elem().Set(reflect.ValueOf(in))

			return []reflect.Value{out.Elem()}
		}).Interface()
	}

	inType := reflect.TypeOf(in)

	ins := make([]reflect.Type, inType.NumIn())
	outs := make([]reflect.Type, inType.NumOut())

	for i := range ins {
		ins[i] = inType.In(i)
	}
	outs[0] = outType.Elem()
	for i := range outs[1:] {
		outs[i+1] = inType.Out(i + 1)
	}

	ctype := reflect.FuncOf(ins, outs, false)

	return reflect.MakeFunc(ctype, func(args []reflect.Value) (results []reflect.Value) {
		outs := reflect.ValueOf(in).Call(args)

		out := reflect.New(outType.Elem())
		if outs[0].Type().AssignableTo(outType.Elem()) {
			// Out: Iface = In: *Struct; Out: Iface = In: OtherIface
			out.Elem().Set(outs[0])
		} else {
			// Out: Iface = &(In: Struct)
			t := reflect.New(outs[0].Type())
			t.Elem().Set(outs[0])
			out.Elem().Set(t)
		}
		outs[0] = out.Elem()

		return outs
	}).Interface()
}

func Override(typ, constructor interface{}) fx.Option {
	if _, ok := typ.(invoke); ok {
		return fx.Invoke(constructor)
	}

	if _, ok := typ.(special); ok {
		return fx.Provide(constructor)
	}
	ctor := as(constructor, typ)
	return fx.Provide(ctor)
}

type NsNetWorkerName = dtypes.NetworkName

var LIBP2PONLY = []fx.Option{

	Override(new(context.Context), context.Background()),

	Override(special{0}, lp2p.DefaultTransports),
	Override(special{1}, lp2p.AddrsFactory(nil, nil)),
	Override(special{2}, lp2p.SmuxTransport(true)),
	Override(special{3}, lp2p.NoRelay()),
	Override(special{4}, lp2p.Security(true, false)),
	Override(special{5}, func(infos []peer.AddrInfo) (lp2p.Libp2pOpts, error) {
		cm := connmgr.NewConnManager(50, 200, 20*time.Second)
		for _, info := range infos {
			cm.Protect(info.ID, "config-prot")
		}
		return lp2p.Libp2pOpts{
			Opts: []libp2p.Option{libp2p.ConnectionManager(cm)},
		}, nil
	}),

	Override(new(lp2p.RawHost), func(ctx context.Context, params P2PHostIn) host.Host {

		opts := []libp2p.Option{
			libp2p.NoListenAddrs,
			libp2p.Ping(true),
		}
		for _, o := range params.Opts {
			opts = append(opts, o...)
		}

		h, err := libp2p.New(ctx, opts...)
		if err != nil {
			log.Error(err)
			return nil
		}
		return h
	}),
	Override(new(host.Host), lp2p.RoutedHost),
	Override(new(lp2p.BaseIpfsRouting), func(ctx context.Context, lc fx.Lifecycle, host lp2p.RawHost, nn dtypes.NetworkName) (lp2p.BaseIpfsRouting, error) {
		log.Infof("NetworkerName %v\n", nn)
		opts := []dht.Option{dht.Mode(dht.ModeAuto),
			dht.ProtocolPrefix(build.DhtProtocolName(nn)),
			dht.QueryFilter(dht.PublicQueryFilter),
			dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			dht.DisableProviders(),
			dht.DisableValues()}
		d, err := dht.New(
			ctx, host, opts...,
		)

		if err != nil {
			return nil, err
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return d.Close()
			},
		})

		return d, nil
	}),

	Override(new([]peer.AddrInfo), func() []peer.AddrInfo {
		infos, err := build.BuiltinBootstrap()
		if err != nil {
			log.Error(err)
			return nil
		}
		return infos
	}),

	Override(invoke(1), func(ctx context.Context, h host.Host, infos []peer.AddrInfo) error {
		for _, info := range infos {
			err := h.Connect(ctx, info)
			log.Error(err)
		}
		return nil
	}),
	Override(invoke(2), func(ctx context.Context, h host.Host, nn dtypes.NetworkName) error {

		// topic := build.MessagesTopic(nn)
		topic := build.BlocksTopic(nn)
		log.Infof("Subscribe topic %s", topic)
		pub, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			return err
		}
		pub.RegisterTopicValidator(topic, func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			blk, err := types.DecodeBlockMsg(msg.GetData())
			if err != nil {
				log.Error(err)
				return pubsub.ValidationReject
			}
			log.Warn("block message validate")
			msg.ValidatorData = blk
			return pubsub.ValidationAccept
		})
		sub, err := pub.Subscribe(topic)
		if err != nil {
			return err
		}

		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				log.Errorf("sub.Next error %v", err)
				continue
			}
			if msg.ValidatorData != nil {
				blk := msg.ValidatorData.(*types.BlockMsg)
				log.Infof("block cid %v height %v", blk.Header.Cid(), blk.Header.Height)
			}
		}
	}),
}

func NewNoDefault(ctx context.Context, ctors ...fx.Option) (node.StopFunc, error) {

	app := fx.New(
		fx.Options(ctors...),
		// fx.NopLogger,
	)

	// TODO: we probably should have a 'firewall' for Closing signal
	//  on this context, and implement closing logic through lifecycles
	//  correctly
	if err := app.Start(ctx); err != nil {
		// comment fx.NopLogger few lines above for easier debugging
		return nil, xerrors.Errorf("starting node: %w", err)
	}

	return app.Stop, nil
}

type Ipv4 string
type Ipv6 string
