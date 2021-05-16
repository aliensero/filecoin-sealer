package up2p

import (
	"context"
	"io"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
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
			node.Override(new(peer.ID), peer.IDFromPublicKey),
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

type MiningCallBackFun func(context.Context, *types.BlockHeader, api.FullNode, address.Address, *util.Key, SealedPath, []*types.SignedMessage) (*util.NsBlockHeader, error)

func HandleIncomingBlocks(mctx helpers.MetricsCtx, lc fx.Lifecycle, ps *pubsub.PubSub, bs dtypes.ChainBlockService, cbs blockstore.Blockstore, h host.Host, nn dtypes.NetworkName, fa api.FullNode, ki *util.Key, actorID address.Address, sp SealedPath, mp *Mpool, cb MiningCallBackFun) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	topic := build.BlocksTopic(nn)
	if err := ps.RegisterTopicValidator(topic, func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		blk, err := types.DecodeBlockMsg(msg.GetData())
		if err != nil {
			log.Error(err)
			return pubsub.ValidationReject
		}
		log.Warn("block message validate")
		msg.ValidatorData = blk
		h.Network().ConnsToPeer(msg.ReceivedFrom)
		log.Infof("conns %v", len(h.Network().Conns()))
		return pubsub.ValidationAccept
	}); err != nil {
		panic(err)
	}

	log.Infof("subscribing to pubsub topic %s", topic)

	blocksub, err := ps.Subscribe(topic) //nolint
	if err != nil {
		panic(err)
	}

	infos, err := build.BuiltinBootstrap()
	if err != nil {
		log.Errorf("failed to get bootstrap peers: %w", err)
		return
	}
	for _, i := range infos {
		err := h.Connect(ctx, i)
		log.Infof("host connect error %v", err)
	}

	curHeight := abi.ChainEpoch(0)
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

		if curHeight >= blk.Header.Height {
			continue
		}
		curHeight = blk.Header.Height
		msgs := mp.Msgs
		go func() {
			defer mp.ClearMsg()
			bh, err := cb(ctx, blk.Header, fa, actorID, ki, sp, msgs)
			if err != nil {
				log.Errorf("MiningCallBackFun error %v", err)
				return
			}
			err = publishBlockMsg(ctx, fa, bh, msgs, ki, ps, topic)
			if err != nil {
				log.Errorf("publishBlockMsg error %v", err)
				return
			}
			bmsgs := make([]*types.Message, 0)
			smsgs := make([]*types.SignedMessage, 0)
			for _, msg := range msgs {
				cm := *msg
				if msg.Signature.Type == crypto.SigTypeBLS {
					bmsgs = append(bmsgs, &cm.Message)
				} else {
					smsgs = append(smsgs, &cm)
				}
			}
			err = SaveBlock(ctx, cbs, bmsgs, smsgs, bh)
			if err != nil {
				log.Errorf("SaveBlock error %v", err)
				return
			}
		}()
	}
}

var ChainSwapOpt = node.Options(
	node.LibP2P,
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
	node.Override(new(MiningCallBackFun), func() MiningCallBackFun {
		return MinerMinng
	}),
	node.Override(node.HandleIncomingBlocksKey, HandleIncomingBlocks),
)

func fetchMessage(ctx context.Context, cbs blockstore.Blockstore, bs dtypes.ChainBlockService, msg *pubsub.Message) {
	blk, ok := msg.ValidatorData.(*types.BlockMsg)
	if !ok {
		log.Warnf("pubsub block validator passed on wrong type: %#v", msg.ValidatorData)
		return
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
		log.Errorf("failed to fetch all bls messages for block received over pubusb: %s; source: %s", err, src)
		return
	}

	smsgs, err := sub.FetchSignedMessagesByCids(ctx, ses, blk.SecpkMessages)
	if err != nil {
		log.Errorf("failed to fetch all secpk messages for block received over pubusb: %s; source: %s", err, src)
		return
	}

	err = SaveBlock(ctx, cbs, bmsgs, smsgs, blk.Header)
	if err != nil {
		log.Error(err)
		return
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
