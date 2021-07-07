package mining

import (
	"context"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"gitlab.ns/lotus-worker/chain"
	"gitlab.ns/lotus-worker/util"
)

var log = logging.Logger("mining")

func init() {
	if v, ok := os.LookupEnv("LOG_LEVEL"); ok {
		logging.SetLogLevel("mining", v)
	} else {
		logging.SetLogLevel("mining", "INFO")
	}
}

func MinerServerNotify(ctx context.Context, fa *chain.P2pLotusAPI, mr util.NsAddress, ki *util.Key, sealedPaths []string) {
	cn, err := chain.NewChainNotify(fa)
	if err != nil {
		log.Error(err)
		return
	}
	notify, err := cn.SubscribeHeader()
	if err != nil {
		log.Error(err)
		return
	}
	for {
		hi := <-notify

		go func() {
			ret, err := MinerMining(ctx, hi, fa.LotusAPI, mr, ki, sealedPaths)
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
}
