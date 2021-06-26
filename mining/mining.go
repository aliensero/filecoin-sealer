package mining

import (
	"context"
	"time"

	"github.com/filecoin-project/lotus/build"
	logging "github.com/ipfs/go-log/v2"
	"gitlab.ns/lotus-worker/util"
)

var log = logging.Logger("mining")

func MinerServerNotify(ctx context.Context, fa *util.P2pLotusAPI, mr util.NsAddress, ki *util.Key, sealedPaths []string) {
	lastHeight := util.NsChainEpoch(0)
	for {
		ts, err := fa.ChainHead(ctx)
		if err != nil {
			log.Errorf("lotus get chain head error %v", err)
			build.Clock.Sleep(3 * time.Second)
			continue
		}
		if lastHeight < ts.Height() {
			go func() {
				ret, err := MinerMining(ctx, ts.Blocks()[0], fa.FullNode, mr, ki, sealedPaths)
				if err != nil {
					log.Errorf("MinerMining error %v", err)
					return
				}
				err = publishBlockMsg(ctx, fa, ret, ki)
				if err != nil {
					log.Errorf("publishBlockMsg error %v", err)
					return
				}
			}()
		}
		lastHeight = ts.Height()
		sleepT := build.Clock.Now().Unix() - int64(ts.MinTimestamp())
		build.Clock.Sleep(time.Duration(sleepT) * time.Second)
	}
}
