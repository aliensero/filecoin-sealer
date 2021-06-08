package up2p

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/blockstore"
	bstore "github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/sub"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/vm"
	"github.com/filecoin-project/lotus/lib/lotuslog"
	"github.com/filecoin-project/lotus/lib/peermgr"
	"github.com/filecoin-project/lotus/metrics"
	"github.com/filecoin-project/lotus/node"
	"github.com/filecoin-project/lotus/node/modules"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	"github.com/filecoin-project/lotus/node/modules/lp2p"
	"github.com/filecoin-project/lotus/node/repo"
	blockadt "github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	cbg "github.com/whyrusleeping/cbor-gen"
	"gitlab.ns/lotus-worker/util"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var log = logging.Logger("util-p2p")

func init() {
	lotuslog.SetupLogLevels()
}

func init() {
	lotuslog.SetupLogLevels()
}

func CreateRepo(path string) (*repo.FsRepo, error) {
	r, err := repo.NewFS(path)
	if err != nil {
		log.Errorf("opening fs repo: %w", err)
		return nil, err
	}
	return r, nil
}

var NsFullNode = repo.FullNode
var NsNodeNew = node.New
var NsOverride = node.Override
var NsApplyIf = node.ApplyIf

type NsSettings = node.Settings

func Repo(r repo.Repo) node.Option {
	return func(settings *node.Settings) error {
		lr, err := r.Lock(repo.FullNode)
		if err != nil {
			return err
		}
		return node.Options(
			node.Override(new(repo.LockedRepo), modules.LockedRepo(lr)),
			node.Override(new(types.KeyStore), modules.KeyStore),
			node.Override(new(ci.PrivKey), lp2p.PrivKey),
			node.Override(new(ci.PubKey), ci.PrivKey.GetPublic),
			node.Override(new(peer.ID), func(pk ci.PubKey) (peer.ID, error) {
				pid, err := peer.IDFromPublicKey(pk)
				log.Infof("PeerID %v", pid)
				return pid, err
			}),
			node.Override(new(dtypes.UniversalBlockstore), func(lc fx.Lifecycle, mctx helpers.MetricsCtx) (dtypes.UniversalBlockstore, error) {
				bs, err := lr.Blockstore(helpers.LifecycleCtx(mctx, lc), repo.UniversalBlockstore)
				if err != nil {
					return nil, err
				}
				if c, ok := bs.(io.Closer); ok {
					lc.Append(fx.Hook{
						OnStop: func(_ context.Context) error {
							return c.Close()
						},
					})
				}
				return bs, nil
			}),
			node.Override(new(dtypes.ChainBlockstore), node.From(new(dtypes.UniversalBlockstore))),
			node.Override(new(dtypes.ExposedBlockstore), node.From(new(dtypes.UniversalBlockstore))),
			node.Override(new(blockstore.Blockstore), node.From(new(dtypes.UniversalBlockstore))),
			node.Override(new(dtypes.MetadataDS), func(lc fx.Lifecycle, mctx helpers.MetricsCtx) (dtypes.MetadataDS, error) {
				mds, err := lr.Datastore(helpers.LifecycleCtx(mctx, lc), "/metadata")
				if err != nil {
					return nil, err
				}
				return mds, nil
			}),
		)(settings)
	}
}

func blackPeers(ps *pubsub.PubSub, h host.Host) {
	if listPath, ok := os.LookupEnv("BLACK_LIST_PATH"); ok {
		f, err := os.Open(listPath)
		if err != nil {
			log.Errorf("open black list path %v error %v", listPath, err)
			return
		}
		buf, err := ioutil.ReadAll(f)
		if err != nil {
			log.Errorf("read black list path %v error %v", listPath, err)
			return
		}
		bls := strings.Split(string(buf), "\n")
		log.Infof("black peers %v", bls)
		for _, bl := range bls {
			if strings.Trim(bl, " ") == "" {
				continue
			}
			badp, err := peer.Decode(bl)
			if err != nil {
				log.Errorf("Decode black peer %v error %v", bl, err)
				continue
			}
			ps.BlacklistPeer(badp)
			h.ConnManager().TagPeer(badp, "badblock", -1000)
		}
	}
}

var ChainSwapOpt = node.Options(
	node.LibP2P,
	node.Override(new(*peermgr.PeerMgr), peermgr.NewPeerMgr),
	node.Override(node.RunPeerMgrKey, func(mctx helpers.MetricsCtx, lc fx.Lifecycle, pmgr *peermgr.PeerMgr, ps *pubsub.PubSub, h host.Host) {
		blackPeers(ps, h)
		modules.RunPeerMgr(mctx, lc, pmgr)
	}),
	node.Override(new(dtypes.NetworkName), func() dtypes.NetworkName {
		return NATNAME
	}),
	node.Override(new(dtypes.DrandSchedule), modules.BuiltinDrandConfig),
	node.Override(new(dtypes.BootstrapPeers), modules.BuiltinBootstrap),
	node.Override(new(dtypes.DrandBootstrap), modules.DrandBootstrap),
	node.Override(new(dtypes.ChainBitswap), modules.ChainBitswap),
	node.Override(new(dtypes.ChainBlockService), modules.ChainBlockService),
	node.Override(new(api.FullNode), func() (api.FullNode, error) {
		api, _, err := client.NewFullNodeRPC(context.TODO(), LOTUSAPISTR, nil)
		return api, err
	}),
	node.Override(new(PrivateKey), func() PrivateKey {
		return PrivateKey("")
	}),
	node.Override(new(address.Address), func() (address.Address, error) {
		return address.NewIDAddress(0)
	}),
	node.Override(new(*util.Key), func(pri PrivateKey) (*util.Key, error) { return util.GenerateKeyByHexString(string(pri)) }),
	node.Override(new(SealedPath), func() SealedPath {
		return "/data01/alien/t012212"
	}),
	node.Override(new(*Mpool), NewMpool),
	node.Override(new(Faddr), func() Faddr {
		return Faddr("127.0.0.1:4321")
	}),
	node.Override(node.SetGenesisKey, ServerRPC),
	node.Override(node.HandleIncomingBlocksKey, HandleIncomingBlocks),
)

func HandleIncomingBlocks(mctx helpers.MetricsCtx, lc fx.Lifecycle, ps *pubsub.PubSub, h host.Host, nn dtypes.NetworkName, bs dtypes.ChainBlockService, cbs blockstore.Blockstore, fa api.FullNode, ki *util.Key, actorID address.Address, sp SealedPath, mp *Mpool, pem *peermgr.PeerMgr) {

	ctx := helpers.LifecycleCtx(mctx, lc)
	curHeight := abi.ChainEpoch(0)

	topic := build.BlocksTopic(nn)
	if err := ps.RegisterTopicValidator(topic, checkBlockMessage(h, pem, &curHeight)); err != nil {
		panic(err)
	}

	log.Infof("subscribing to pubsub topic %s", topic)

	blocksub, err := ps.Subscribe(topic) //nolint
	if err != nil {
		panic(err)
	}
	subChan := make(chan *pubsub.Message, 10)
	go miningServer(ctx, cbs, bs, subChan, fa, actorID, ki, sp, mp, ps, topic)

	for {
		msg, err := blocksub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Warn("quitting HandleIncomingBlocks loop")
				return
			}
			log.Error("error from block subscription: ", err)
			continue
		}

		blk, ok := msg.ValidatorData.(*types.BlockMsg)
		if !ok {
			log.Warnf("pubsub block validator passed on wrong type: %#v", msg.ValidatorData)
			continue
		}

		if curHeight > blk.Header.Height {
			continue
		}
		curHeight = blk.Header.Height
		subChan <- msg
	}
}

func miningServer(ctx context.Context, cbs blockstore.Blockstore, bs dtypes.ChainBlockService, pms chan *pubsub.Message, fa util.LotusAPI, mr util.NsAddress, ki *util.Key, sealedPath SealedPath, mp *Mpool, pub *pubsub.PubSub, topic string) {
	delay := time.Duration(build.BlockDelaySecs) * time.Second
	lastHeight := abi.ChainEpoch(0)
	for {
		mapMsg := make(map[util.NsCid]*util.NsMessage)
		mapHead := make(map[util.NsCid][]cid.Cid)
		tt := time.NewTimer(delay)
		var h *types.BlockHeader
	loop:
		for {
			select {
			case pm := <-pms:

				if blk, ok := pm.ValidatorData.(*types.BlockMsg); ok {

					if lastHeight >= blk.Header.Height {
						continue
					}

					if h != nil && blk.Header.Height > h.Height {
						pms <- pm
						break loop
					}

					if h == nil {
						h = blk.Header
						tt.Reset(delay / 3)
					}

					// if _, ok := mapHead[blk.Cid()]; !ok {
					// 	cidS := append(blk.BlsMessages, blk.SecpkMessages...)
					// 	mapHead[blk.Cid()] = cidS
					// 	for _, c := range cidS {
					// 		if _, ok := mapMsg[c]; !ok {
					// 			var err error
					// 			var s *types.Message
					// 			bf, err := mp.Cache.Get(c)
					// 			if err != nil {
					// 				log.Warnf("mpool get bls message %v error %v", c, err)
					// 			} else {
					// 				s, err = types.DecodeMessage(bf.([]byte))
					// 				if err != nil {
					// 					log.Warnf("mpool DecodeSignedMessage bls message %v error %v", c, err)
					// 				}
					// 			}
					// 			// if err != nil {
					// 			// 	ss, terr := FetchSigMsgByCid(ctx, cbs, bs, c)
					// 			// 	if terr != nil || len(ss) == 0 {
					// 			// 		ss, terr := FetchMsgByCid(ctx, cbs, bs, c)
					// 			// 		if terr != nil {
					// 			// 			err = xerrors.Errorf("FetchSigMsgByCid cid %v len %v error %v", c, len(ss), terr)
					// 			// 		} else {
					// 			// 			s = ss[0]
					// 			// 		}
					// 			// 	} else {
					// 			// 		err = nil
					// 			// 		s = &ss[0].Message
					// 			// 		buf, terr := s.Serialize()
					// 			// 		if terr == nil {
					// 			// 			mp.Cache.Set(c, buf)
					// 			// 		}
					// 			// 	}
					// 			// }
					// 			if err != nil {
					// 				log.Errorf("mpool get message %v error %v", c, err)
					// 				delete(mapHead, blk.Cid())
					// 				break loop
					// 			}
					// 			mapMsg[c] = s
					// 		}
					// 	}
					// }
				}

			case <-tt.C:
				break loop
			}
		}

		if h == nil {
			continue
		}
		lastHeight = h.Height

		if uint64(build.Clock.Now().Unix())-h.Timestamp >= build.BlockDelaySecs {
			log.Warnf("fetch message delay")
			continue
		}

		go func() {
			_, err := MinerMining(ctx, h, mapHead, fa, mr, ki, sealedPath, mapMsg, func(parentMsg []*util.NsMessage, mret *MiningResult) error {
				err := publishBlockMsg(ctx, fa, mret, cbs, parentMsg, ki, pub, topic)
				if err != nil {
					log.Errorf("publishBlockMsg error %v", err)
					return err
				}
				return nil
			})
			if err != nil {
				log.Errorf("MiningCallBackFun error %v", err)
				return
			}
		}()
	}
}

func checkBlockMessage(h host.Host, pem *peermgr.PeerMgr, curHeight *abi.ChainEpoch) func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	return func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		blk, err := types.DecodeBlockMsg(msg.GetData())
		if err != nil {
			log.Error(err)
			return pubsub.ValidationReject
		}
		log.Warn("block message validate")
		msg.ValidatorData = blk
		h.Network().ConnsToPeer(msg.ReceivedFrom)
		log.Infof("conns %v", len(h.Network().Conns()))
		pem.AddFilecoinPeer(pid)
		if blk.Header.Height >= *curHeight {
			h.ConnManager().TagPeer(pid, "new-block", 40)
		}
		return pubsub.ValidationAccept
	}
}

func checkIncomingMessage(h host.Host) func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	return func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		smsg, err := types.DecodeSignedMessage(msg.GetData())
		if err != nil {
			log.Error(err)
			return pubsub.ValidationReject
		}
		msg.ValidatorData = smsg
		return pubsub.ValidationAccept
	}
}

func fetchMessage(ctx context.Context, cbs blockstore.Blockstore, bs dtypes.ChainBlockService, msg *pubsub.Message) ([]*types.Message, error) {
	retMsg := make([]*types.Message, 0)
	blk, ok := msg.ValidatorData.(*types.BlockMsg)
	if !ok {
		return retMsg, xerrors.Errorf("pubsub block validator passed on wrong type: %#v", msg.ValidatorData)
	}
	src := msg.GetFrom()
	timeout := time.Duration(build.BlockDelaySecs+build.PropagationDelaySecs) * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// NOTE: we could also share a single session between
	// all requests but that may have other consequences.
	ses := bserv.NewSession(ctx, bs)

	start := build.Clock.Now()
	log.Debug("about to fetch messages for block from pubsub")
	bmsgs, err := sub.FetchMessagesByCids(ctx, ses, blk.BlsMessages)
	if err != nil {
		return retMsg, xerrors.Errorf("failed to fetch all bls messages for block received over pubusb: %s; source: %s", err, src)
	}
	retMsg = append(retMsg, bmsgs...)

	smsgs, err := sub.FetchSignedMessagesByCids(ctx, ses, blk.SecpkMessages)
	if err != nil {
		return retMsg, xerrors.Errorf("failed to fetch all secpk messages for block received over pubusb: %s; source: %s", err, src)
	}
	for _, sm := range smsgs {
		retMsg = append(retMsg, &sm.Message)
	}

	err = SaveBlock(ctx, cbs, bmsgs, smsgs, blk.Header)
	if err != nil {
		log.Warn(err)
	}

	took := build.Clock.Since(start)
	log.Debugw("new block over pubsub", "cid", blk.Header.Cid(), "source", msg.GetFrom(), "msgfetch", took)
	if took > 3*time.Second {
		log.Warnw("Slow msg fetch", "cid", blk.Header.Cid(), "source", msg.GetFrom(), "msgfetch", took)
	}
	if delay := build.Clock.Now().Unix() - int64(blk.Header.Timestamp); delay > 5 {
		_ = stats.RecordWithTags(ctx,
			[]tag.Mutator{tag.Insert(metrics.MinerID, blk.Header.Miner.String())},
			metrics.BlockDelay.M(delay),
		)
		log.Warnw("received block with large delay from miner", "block", blk.Cid(), "delay", delay, "miner", blk.Header.Miner)
	}
	return retMsg, nil
}

func FetchMsgByCid(ctx context.Context, cbs blockstore.Blockstore, bs dtypes.ChainBlockService, c cid.Cid) ([]*types.Message, error) {
	ses := bserv.NewSession(ctx, bs)
	log.Debug("about to fetch messages for block from pubsub")
	msgs, err := sub.FetchMessagesByCids(ctx, ses, []cid.Cid{c})
	if err != nil {
		log.Errorf("failed to fetch all bls messages for block received over pubusb: %s", err)
		return nil, err
	}
	return msgs, nil
}

func FetchSigMsgByCid(ctx context.Context, cbs blockstore.Blockstore, bs dtypes.ChainBlockService, c cid.Cid) ([]*types.SignedMessage, error) {
	ses := bserv.NewSession(ctx, bs)
	log.Debug("about to fetch signedmessages for block from pubsub")
	msgs, err := sub.FetchSignedMessagesByCids(ctx, ses, []cid.Cid{c})
	if err != nil {
		log.Errorf("failed to fetch all bls signedmessages for block received over pubusb: %s", err)
		return nil, err
	}
	return msgs, nil
}

func SaveBlock(ctx context.Context, cbs blockstore.Blockstore, bmsgs []*types.Message, smsgs []*types.SignedMessage, head *types.BlockHeader) error {
	blockstore := bstore.NewMemory()
	cst := cbor.NewCborStore(blockstore)

	var bcids, scids []cid.Cid

	for _, m := range bmsgs {
		c, err := store.PutMessage(blockstore, m)
		if err != nil {
			return xerrors.Errorf("putting bls message to blockstore after msgmeta computation: %w", err)
		}
		bcids = append(bcids, c)
	}

	for _, m := range smsgs {
		c, err := store.PutMessage(blockstore, m)
		if err != nil {
			return xerrors.Errorf("putting bls message to blockstore after msgmeta computation: %w", err)
		}
		scids = append(scids, c)
	}
	smroot, err := computeMsgMeta(cst, bcids, scids)
	if err != nil {
		return xerrors.Errorf("validating msgmeta, compute failed: %w", err)
	}

	vm.Copy(ctx, blockstore, cbs, smroot)

	sb, err := head.ToStorageBlock()
	if err != nil {
		return xerrors.Errorf("ToStorageBlock: %v", err)
	}
	err = cbs.Put(sb)
	if err != nil {
		return xerrors.Errorf("chainstore put: %v", err)
	}
	log.Infof("block cid %v", sb.Cid())
	return nil
}

func computeMsgMeta(bs cbor.IpldStore, bmsgCids, smsgCids []cid.Cid) (cid.Cid, error) {
	// block headers use adt0
	store := blockadt.WrapStore(context.TODO(), bs)
	bmArr := blockadt.MakeEmptyArray(store)
	smArr := blockadt.MakeEmptyArray(store)

	for i, m := range bmsgCids {
		c := cbg.CborCid(m)
		if err := bmArr.Set(uint64(i), &c); err != nil {
			return cid.Undef, err
		}
	}

	for i, m := range smsgCids {
		c := cbg.CborCid(m)
		if err := smArr.Set(uint64(i), &c); err != nil {
			return cid.Undef, err
		}
	}

	bmroot, err := bmArr.Root()
	if err != nil {
		return cid.Undef, err
	}

	smroot, err := smArr.Root()
	if err != nil {
		return cid.Undef, err
	}

	mrcid, err := store.Put(store.Context(), &types.MsgMeta{
		BlsMessages:   bmroot,
		SecpkMessages: smroot,
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to put msgmeta: %w", err)
	}

	return mrcid, nil
}
