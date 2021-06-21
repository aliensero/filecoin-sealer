package mining

import (
	"context"
	"time"

	"github.com/filecoin-project/lotus/build"
	logging "github.com/ipfs/go-log/v2"
	"gitlab.ns/lotus-worker/util"
)

var log = logging.Logger("mining")

func MinerServerNotify(ctx context.Context, fa util.LotusAPI, mr util.NsAddress, ki *util.Key, sealedPaths []string) {
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
				_, err := MinerMining(ctx, ts.Blocks()[0], fa, mr, ki, sealedPaths)
				if err != nil {
					log.Errorf("MiningCallBackFun error %v", err)
					return
				}
			}()
		}
		lastHeight = ts.Height()
		sleepT := build.Clock.Now().Unix() - int64(ts.MinTimestamp())
		build.Clock.Sleep(time.Duration(sleepT) * time.Second)
	}
}
