package up2p

import (
	"context"

	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/sub"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/lib/peermgr"
	"github.com/filecoin-project/lotus/node"
	"github.com/filecoin-project/lotus/node/modules"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"gitlab.ns/lotus-worker/util"
	"go.uber.org/fx"
)

type DbParams struct {
	User     string
	Password string
	Ip       string
	Port     string
	Database string
}

var CollectTtranOpt = node.Options(
	node.LibP2P,
	node.Override(new(*peermgr.PeerMgr), peermgr.NewPeerMgr),
	node.Override(node.RunPeerMgrKey, modules.RunPeerMgr),
	node.Override(new(dtypes.NetworkName), func() dtypes.NetworkName {
		return NATNAME
	}),
	node.Override(new(dtypes.DrandSchedule), modules.BuiltinDrandConfig),
	node.Override(new(dtypes.BootstrapPeers), modules.BuiltinBootstrap),
	node.Override(new(dtypes.DrandBootstrap), modules.DrandBootstrap),
	node.Override(new(blockstore.Blockstore), func() blockstore.Blockstore {
		return blockstore.NewMemory()
	}),
	node.Override(new(dtypes.ExposedBlockstore), node.From(new(blockstore.Blockstore))),
	node.Override(new(dtypes.ChainBitswap), modules.ChainBitswap),
	node.Override(new(DbParams), func() DbParams {
		return DbParams{
			User:     "root",
			Password: "123456",
			Ip:       "127.0.0.1",
			Port:     "3306",
			Database: "db_worker",
		}
	}),
	node.Override(node.HandleIncomingBlocksKey, HandleCollectTran),
)

func HandleCollectTran(mctx helpers.MetricsCtx, lc fx.Lifecycle, dbparams DbParams, ps *pubsub.PubSub, h host.Host, nn dtypes.NetworkName, bs blockstore.Blockstore, rem dtypes.ChainBitswap) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	topic := build.BlocksTopic(nn)
	if err := ps.RegisterTopicValidator(topic, checkBlockMessage(h)); err != nil {
		panic(err)
	}

	log.Infof("subscribing to pubsub topic %s", topic)

	blocksub, err := ps.Subscribe(topic) //nolint
	if err != nil {
		panic(err)
	}

	db, err := util.InitMysql(dbparams.User, dbparams.Password, dbparams.Ip, dbparams.Port, dbparams.Database)
	if err != nil {
		panic(err)
	}

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
		go func() {
			msgs, err := sub.FetchMessagesByCids(ctx, getMemBlockServiceSession(ctx, bs, rem), blk.BlsMessages)
			if err != nil {
				return
			}
			for _, m := range msgs {
				ti := util.DbTransaction{
					Cid:     m.Cid().String(),
					From:    m.From.String(),
					To:      m.To.String(),
					Value:   m.Value.String(),
					Method:  uint64(m.Method),
					NetName: NATNAME,
				}
				if err := db.Where(ti).FirstOrCreate(&ti).Error; err != nil {
					log.Errorf("db create %v error %v", ti, err)
				}
			}
		}()
		go func() {
			msgs, err := sub.FetchSignedMessagesByCids(ctx, getMemBlockServiceSession(ctx, bs, rem), blk.SecpkMessages)
			if err != nil {
				return
			}
			for _, m := range msgs {
				ti := util.DbTransaction{
					Cid:     m.Cid().String(),
					From:    m.Message.From.String(),
					To:      m.Message.To.String(),
					Value:   m.Message.Value.String(),
					Method:  uint64(m.Message.Method),
					NetName: NATNAME,
				}
				if err := db.Where(ti).FirstOrCreate(&ti).Error; err != nil {
					log.Errorf("db create %v error %v", ti, err)
				}
			}
		}()
	}
}

func getMemBlockServiceSession(ctx context.Context, bs blockstore.Blockstore, rem dtypes.ChainBitswap) *bserv.Session {
	bsservice := bserv.New(bs, rem)
	return bserv.NewSession(ctx, bsservice)
}
