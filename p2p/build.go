package p2p

import (
	"bytes"
	"context"
	"fmt"
	"os"
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

var set map[interface{}]interface{}

func init() {
	set = make(map[interface{}]interface{})
}

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
	Override(invoke(2), func(ctx context.Context, h host.Host, nn util.NsNetworkName, mr util.NsAddress, prihex PriHex, fa util.LotusAPI, path SealedPath) error {
		// topic := build.MessagesTopic(nn)
		topic := util.NsBlocksTopic(nn)
		log.Infof("Subscribe topic %s", topic)
		pub, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			return err
		}

		blkChan := make(chan *util.NsBlockMsg, 5)
		go func() {
			curH := util.NsChainEpoch(0)
			for {
				select {
				case b := <-blkChan:
					cpb := b
					if curH >= cpb.Header.Height {
						continue
					}
					curH = cpb.Header.Height
					go MinerMinng(ctx, cpb.Header, fa, mr, string(prihex), string(path), pub, topic)
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
type SealedPath string

func MinerMinng(ctx context.Context, blkh *util.NsBlockHeader, fa util.LotusAPI, mr util.NsAddress, prihex string, sealedPath string, pub *pubsub.PubSub, topic string) {
	curTipset, err := fa.ChainHead(ctx)
	if err != nil {
		log.Errorf("MinerMinng ChainHead error %v", err)
		return
	}
	mbi, round, err := getBaseInfo(ctx, blkh, fa, mr, curTipset)
	if err != nil || mbi == nil {
		log.Errorf("MinerMinng get miner info mbi == nil %v error %v", mbi == nil, err)
		return
	}

	ki, err := util.GenerateKeyByHexString(prihex)
	if err != nil {
		log.Errorf("MinerMinng GenerateKeyByHexString %v", err)
		return
	}

	bvals := mbi.BeaconEntries
	rbase := mbi.PrevBeaconEntry
	if len(bvals) > 0 {
		rbase = bvals[len(bvals)-1]
	}

	electionRand, err := getElectionRand(mr, rbase.Data, round)
	if err != nil {
		log.Errorf("MinerMinng failed to draw randomness: %v", err)
		return
	}

	minerPower := mbi.MinerPower
	ticket, ep, err := getTicketAndElectionProof(ctx, mbi, electionRand, mr, round, ki, minerPower)
	if err != nil || ep.WinCount < 1 {
		log.Errorf("MinerMinng getTicketAndElectionProof wincount %v error %v", ep.WinCount, err)
		return
	}

	msgs, err := fa.MpoolSelect(ctx, curTipset.Key(), ticket.Quality())
	if err != nil {
		log.Errorf("MinerMinng select msgs error %v", err)
		return
	}

	actorID, err := util.NsIDFromAddress(mr)
	if err != nil {
		log.Errorf("MinerMinng NsIDFromAddress miner %v error %v", mr, err)
		return
	}

	proofType, err := util.NsWinningPoStProofTypeFromWindowPoStProofType(255, util.NsRegisteredPoStProof(mbi.Sectors[0].SealProof))
	if err != nil {
		log.Error("MinerMinng determining winning post proof type: %v", err)
		return
	}

	wpostProof, err := util.GenerateWinningPoSt(ctx, util.NsActorID(actorID), mbi.Sectors, electionRand, proofType, sealedPath)
	if err != nil {
		log.Errorf("MinerMinng GenerateWinningPoSt miner %v error %v", mr, err)
		return
	}

	uts := curTipset.MinTimestamp() + util.NsBlockDelaySecs

	blkHead := &util.NsBlockHeader{
		Miner:         mr,
		Parents:       curTipset.Key().Cids(),
		Ticket:        ticket,
		ElectionProof: ep,

		BeaconEntries:         mbi.BeaconEntries,
		Height:                round,
		Timestamp:             uts,
		WinPoStProof:          wpostProof,
		ParentStateRoot:       curTipset.Cids()[0],
		ParentMessageReceipts: curTipset.Cids()[1],
	}

	err = publishBlockMsg(ctx, curTipset, fa, blkHead, msgs, ki, pub, topic)
	if err != nil {
		log.Errorf("MinerMinng publishBlockMsg miner %v error %v", mr, err)
	}

}

func getBaseInfo(ctx context.Context, blkh *util.NsBlockHeader, fa util.LotusAPI, mr util.NsAddress, curTipset *util.NsTipSet) (*util.NsMiningBaseInfo, util.NsChainEpoch, error) {
	tps, err := util.NsNewTipSet([]*util.NsBlockHeader{blkh})
	if err != nil {
		return nil, 0, err
	}
	round := tps.Height() + util.NsChainEpoch(1)
	mbi, err := fa.MinerGetBaseInfo(ctx, mr, round, curTipset.Key())
	return mbi, round, err
}

func getElectionRand(mr util.NsAddress, rebaseData []byte, round util.NsChainEpoch) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := mr.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	electionRand, err := util.NsDrawRandomness(rebaseData, util.NsDomainSeparationTag_ElectionProofProduction, round, buf.Bytes())
	if err != nil {
		return nil, err
	}
	return electionRand, nil
}

func getTicketAndElectionProof(ctx context.Context, mbi *util.NsMiningBaseInfo, electionRand []byte, mr util.NsAddress, round util.NsChainEpoch, ki *util.Key, minerPower util.Nsbig) (*util.NsTicket, *util.NsElectionProof, error) {

	vrfout, err := util.NsComputeVRF(ctx, func(ctx context.Context, addr util.NsAddress, data []byte) (*util.NsSignature, error) {
		sigture, err := util.SignMsg(ki.NsKeyInfo, electionRand)
		if err != nil {
			return nil, err
		}
		return sigture, nil
	}, util.NsAddress{}, nil)
	if err != nil {
		return nil, nil, err
	}

	ep := &util.NsElectionProof{VRFProof: vrfout}
	j := ep.ComputeWinCount(minerPower, mbi.NetworkPower)
	ep.WinCount = j
	log.Infof("wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww round %v curPower %v NetworkPower %v sectors %v count %d", round, minerPower, mbi.NetworkPower, mbi.Sectors, ep.WinCount)

	ticket := &util.NsTicket{
		VRFProof: vrfout,
	}
	return ticket, ep, nil
}

func publishBlockMsg(ctx context.Context, curTipset *util.NsTipSet, fa util.LotusAPI, blkHead *util.NsBlockHeader, msgs []*util.NsSignedMessage, ki *util.Key, pub *pubsub.PubSub, topic string) error {
	var blk util.NsBlockMsg
	var blsSigs []util.NsSignature

	blockstore := util.NsNewMemory()
	bs := util.NsNewCborStore(blockstore)
	var blkcids []util.NsCid
	var secpkcids []util.NsCid
	for _, msg := range msgs {
		if msg.Signature.Type == util.NsSigTypeBLS {
			blsSigs = append(blsSigs, msg.Signature)
			blk.BlsMessages = append(blk.BlsMessages, msg.Cid())
			cid, err := bs.Put(ctx, msg.Message)
			if err != nil {
				return xerrors.Errorf("publishBlockMsg blockstore error %v", err)
			}
			blkcids = append(blkcids, cid)
		} else {
			blk.SecpkMessages = append(blk.SecpkMessages, msg.Cid())
			cid, err := bs.Put(ctx, msg)
			if err != nil {
				return xerrors.Errorf("publishBlockMsg blockstore error %v", err)
			}
			secpkcids = append(secpkcids, cid)
		}
	}

	blsmsgroot, err := util.NsComputeMsgMeta(bs, blkcids, secpkcids)
	if err != nil {
		return xerrors.Errorf("publishBlockMsg NsComputeMsgMeta error %v", err)
	}
	blkHead.Messages = blsmsgroot

	parentWeight, err := fa.ChainTipSetWeight(ctx, curTipset.Key())
	if err != nil {
		return err
	}
	blkHead.ParentWeight = parentWeight

	baseFee, err := util.NsComputeBaseFee(ctx, fa, curTipset)
	if err != nil {
		return err
	}
	blkHead.ParentBaseFee = baseFee

	aggSig, err := util.NsaggregateSignatures(blsSigs)
	if err != nil {
		return err
	}
	blkHead.BLSAggregate = aggSig

	hbuf, err := blkHead.Serialize()
	if err != nil {
		return err
	}
	sigture, err := util.SignMsg(ki.NsKeyInfo, hbuf)
	if err != nil {
		return err
	}
	blkHead.BlockSig = sigture

	blk.Header = blkHead

	b, err := blk.Serialize()
	if err != nil {
		return err
	}
	err = pub.Publish(topic, b)
	if err != nil {
		return err
	}
	log.Infof("publishBlockMsg cid %v", blsmsgroot)
	return nil
}

func GenerateWinningFallbackSectorChallenges(mr util.NsAddress, rebaseData []byte, round util.NsChainEpoch, mbi *util.NsMiningBaseInfo) (*util.NsFallbackChallenges, error) {
	rand, err := getElectionRand(mr, rebaseData, round)
	if err != nil {
		return nil, err
	}
	proofType, err := util.NsWinningPoStProofTypeFromWindowPoStProofType(255, util.NsRegisteredPoStProof(mbi.Sectors[0].SealProof))
	if err != nil {
		return nil, err
	}
	actorID, err := util.NsIDFromAddress(mr)
	sectorIds := make([]util.NsSectorNum, len(mbi.Sectors))
	for i, s := range mbi.Sectors {
		sectorIds[i] = s.SectorNumber
	}
	return util.NsGeneratePoStFallbackSectorChallenges(proofType, util.NsActorID(actorID), rand, sectorIds)
}

func GenerateSingleWinningVanillaProof(
	path string,
	mr util.NsAddress,
	poStProofType util.NsRegisteredPoStProof,
	rebaseData []byte, round util.NsChainEpoch,
	mbi *util.NsMiningBaseInfo,
) (map[util.NsSectorNum][]byte, error) {
	actorID, err := util.NsIDFromAddress(mr)
	if err != nil {
		return nil, err
	}
	challages, err := GenerateWinningFallbackSectorChallenges(mr, rebaseData, round, mbi)
	if err != nil {
		return nil, err
	}
	retMap := make(map[util.NsSectorNum][]byte)
	for _, si := range mbi.Sectors {
		_, err := os.Stat(fmt.Sprintf("%s/cache/s-t0%d-%d/p_aux", path, actorID, si.SectorNumber))
		if err != nil {
			continue
		}
		privateSectorInfo := util.NsPrivateSectorInfo{}
		privateSectorInfo.SectorInfo = util.NsSectorInfo{
			SealProof:    si.SealProof,
			SectorNumber: si.SectorNumber,
			SealedCID:    si.SealedCID,
		}
		privateSectorInfo.CacheDirPath = fmt.Sprintf("%s/cache/s-t0%d-%d", path, actorID, si.SectorNumber)
		privateSectorInfo.PoStProofType = poStProofType
		privateSectorInfo.SealedSectorPath = fmt.Sprintf("%s/sealed/s-t0%d-%d", path, actorID, si.SectorNumber)
		vp, err := util.NsGenerateSingleVanillaProof(privateSectorInfo, challages.Challenges[si.SectorNumber])
		if err != nil {
			return nil, err
		}
		retMap[si.SectorNumber] = vp
	}
	return retMap, nil
}

func GenerateWinningPoStWithVanilla(
	proofType util.NsRegisteredPoStProof,
	minerID util.NsActorID,
	randomness []byte,
	proofs [][]byte,
) {

}
