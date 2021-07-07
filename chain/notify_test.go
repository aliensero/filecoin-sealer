package chain

import (
	"testing"
	"time"

	"gitlab.ns/lotus-worker/util"
)

func TestChainNotify(t *testing.T) {
	t.Log("test begin")

	fa, close, err := util.NewPubLotusApi("https://filestar.info/rpc/v0")
	if err != nil {
		t.Error(err)
	}
	defer close()
	ffa, err := NewP2pLotusAPI(fa)
	if err != nil {
		t.Error(err)
	}
	c, err := NewChainNotify(ffa)
	if err != nil {
		t.Error(err)
	}

	ret, err := c.SubscribeHeader()
	if err != nil {
		t.Error(err)
	}

	for {
		h := <-ret
		log.Info(h.Height, " ", len(h.Message), " ", time.Now().Unix()-int64(h.CurTipSet.MinTimestamp()))
	}
}
