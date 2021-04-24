package p2p

import (
	"bytes"
	"context"
	"reflect"
	"time"

	"gitlab.ns/lotus-worker/util"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
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

var LIBP2PONLY = []fx.Option{

	Override(new(context.Context), context.Background()),

	Override(special{0}, util.NsDefaultTransports),
	Override(special{1}, util.NsAddrsFactory(nil, nil)),
	Override(special{2}, util.NsSmuxTransport(true)),
	Override(special{3}, util.NsNoRelay()),
	Override(special{4}, util.NsSecurity(true, false)),
	Override(special{5}, func(infos []peer.AddrInfo) (util.NsLibp2pOpts, error) {
		cm := connmgr.NewConnManager(50, 200, 20*time.Second)
		for _, info := range infos {
			cm.Protect(info.ID, "config-prot")
		}
		return util.NsLibp2pOpts{
			Opts: []libp2p.Option{libp2p.ConnectionManager(cm)},
		}, nil
	}),
	// Override(new(util.LotusApiStr), "https://calibration.node.glif.io"),
	Override(new(util.LotusAPI), util.NewPubLotusApi1),
	// Override(new(AddrStr), AddrStr("t01000")),
	Override(new(util.NsAddress), func(as AddrStr) util.NsAddress {
		a, err := util.NsNewFromString(string(as))
		if err != nil {
			panic(err)
		}
		return a
	}),

	Override(new(util.NsRawHost), func(ctx context.Context, params P2PHostIn) host.Host {

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
	Override(new(host.Host), util.NsRoutedHost),
	Override(new(util.NsBaseIpfsRouting), func(ctx context.Context, lc fx.Lifecycle, host util.NsRawHost, nn util.NsNetworkName) (util.NsBaseIpfsRouting, error) {
		log.Infof("NetworkerName %v\n", nn)
		opts := []dht.Option{dht.Mode(dht.ModeAuto),
			dht.ProtocolPrefix(util.NsDhtProtocolName(nn)),
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
		infos, err := util.NsBuiltinBootstrap()
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
	Override(invoke(2), func(ctx context.Context, h host.Host, nn util.NsNetworkName, mr util.NsAddress, prihex PriHex, testPower Power, fa util.LotusAPI) error {
		// topic := build.MessagesTopic(nn)
		topic := util.NsBlocksTopic(nn)
		log.Infof("Subscribe topic %s", topic)
		pub, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			return err
		}

		var IsWinnerFunc = func(blkh *util.NsBlockHeader) {
			tps, err := util.NsNewTipSet([]*util.NsBlockHeader{blkh})
			if err != nil {
				log.Errorf("NewTipSet error %v", err)
				return
			}
			round := tps.Height() + util.NsChainEpoch(1)
			msgs, err := fa.MpoolSelect(ctx, tps.Key(), 0.81)
			if len(msgs) > 0 {
				log.Infof("select msgs %v error %v", msgs[0], err)
			}
			curTipset, err := fa.ChainHead(ctx)
			if err != nil {
				log.Errorf("ChainGetTipSeterror %v", err)
				return
			}
			mbi, err := fa.MinerGetBaseInfo(ctx, mr, curTipset.Height(), curTipset.Key())
			if err != nil || mbi == nil {
				log.Errorf("get miner info mbi == nil %v error %v", mbi == nil, err)
				return
			}
			beaconPrev := mbi.PrevBeaconEntry
			bvals := mbi.BeaconEntries
			rbase := beaconPrev
			if len(bvals) > 0 {
				rbase = bvals[len(bvals)-1]
			}

			buf := new(bytes.Buffer)
			if err := mr.MarshalCBOR(buf); err != nil {
				log.Error(xerrors.Errorf("failed to cbor marshal address: %w", err))
				return
			}

			electionRand, err := util.NsDrawRandomness(rbase.Data, util.NsDomainSeparationTag_ElectionProofProduction, round, buf.Bytes())
			if err != nil {
				log.Error(xerrors.Errorf("failed to draw randomness: %w", err))
				return
			}
			log.Infof("blk cid %v worker key %v electionRand %v NetworkPower %v", blkh.Cid(), mbi.WorkerKey, electionRand, mbi.NetworkPower)
			if prihex != "" {
				vrfout, err := util.NsComputeVRF(ctx, func(ctx context.Context, addr util.NsAddress, data []byte) (*util.NsSignature, error) {
					ki, err := util.GenerateKeyByHexString(string(prihex))
					if err != nil {
						return nil, err
					}
					sigture, err := util.SignMsg(ki.NsKeyInfo, electionRand)
					if err != nil {
						return nil, err
					}
					return sigture, nil
				}, mbi.WorkerKey, electionRand)
				if err != nil {
					log.Error(xerrors.Errorf("failed to compute VRF: %w", err))
					return
				}

				ep := &util.NsElectionProof{VRFProof: vrfout}
				j := ep.ComputeWinCount(util.NsNewInt(uint64(testPower)), mbi.NetworkPower)
				// ep.WinCount = j
				log.Infof("wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww testPower %v NetworkPower %v count %d", testPower, mbi.NetworkPower, j)
			}
		}

		blkChan := make(chan *util.NsBlockMsg, 5)
		go func() {
			for {
				select {
				case b := <-blkChan:
					cpb := b
					go IsWinnerFunc(cpb.Header)
				}
			}
		}()

		pub.RegisterTopicValidator(topic, func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			blk, err := util.NsDecodeBlockMsg(msg.GetData())
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
		var tmpHeight util.NsChainEpoch = 0
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				log.Errorf("sub.Next error %v", err)
				continue
			}
			if msg.ValidatorData != nil {
				blk := msg.ValidatorData.(*util.NsBlockMsg)
				if blk.Header.Height >= tmpHeight {
					blkChan <- blk
					tmpHeight = blk.Header.Height
				}
				log.Infof("block cid %v height %v", blk.Header.Cid(), blk.Header.Height)
			}
		}
	}),
}

func NewNoDefault(ctx context.Context, ctors ...fx.Option) (func(context.Context) error, error) {

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
type AddrStr string
type PriHex string
type Power uint64
