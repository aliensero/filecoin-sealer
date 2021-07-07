package chain

import (
	"context"
	"os"
	"time"

	"github.com/bluele/gcache"
	logging "github.com/ipfs/go-log/v2"
	"gitlab.ns/lotus-worker/util"
)

var log = logging.Logger("chain")

func init() {
	if v, ok := os.LookupEnv("LOG_LEVEL"); ok {
		logging.SetLogLevel("chain", v)
	} else {
		logging.SetLogLevel("chain", "INFO")
	}
}

type ChainNotify struct {
	ctx         context.Context
	Cache       gcache.Cache
	MessageChan chan *util.NsSignedMessage
	BlockChan   chan []*util.NsBlockMsg
	fa          *P2pLotusAPI
}

type HeadInfo struct {
	Height    util.NsChainEpoch
	CurTipSet util.NsTipSet
	Message   []util.NsMessage
}

func (c *ChainNotify) SubscribeHeader() (chan HeadInfo, error) {
	ret := make(chan HeadInfo)
	c.subscribeMsg()
	c.subscribeBlock()
	go func() {
		for {
			select {
			case sm := <-c.MessageChan:
				err := c.Cache.Set(sm.Cid(), sm.Message)
				if err != nil {
					log.Error(err)
					continue
				}
				log.Debugf("incomming message cid %v", sm.Cid())
			case blks := <-c.BlockChan:

				go func() {
					var headers []*util.NsBlockHeader
					var msgs []util.NsMessage
					var msgsMap = make(map[util.NsCid]struct{})
					for _, blk := range blks {
						for _, mcid := range append(blk.BlsMessages, blk.SecpkMessages...) {
							if _, ok := msgsMap[mcid]; !ok {
								v, err := c.Cache.Get(mcid)
								if err != nil {
									log.Errorf("get cache message cid %v error %v", mcid, err)
									return
								}
								m, ok := v.(util.NsMessage)
								if !ok {
									return
								}
								msgs = append(msgs, m)
								msgsMap[mcid] = struct{}{}
							}
						}
						headers = append(headers, blk.Header)
					}
					if len(headers) > 0 {
						ts, err := util.NsNewTipSet(headers)
						if err != nil {
							return
						}
						height := headers[0].Height
						ret <- HeadInfo{
							Height:    height,
							CurTipSet: *ts,
							Message:   msgs,
						}
					}
				}()

			case <-c.ctx.Done():
				return
			}
		}
	}()
	return ret, nil
}

func (c *ChainNotify) subscribeMsg() {
	ctx := c.ctx
	go func() {
		sub, err := c.fa.GetMsgTopic().Subscribe()
		if err != nil {
			log.Error(err)
			return
		}
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				continue
			}
			smsg, err := util.NsDecodeSignedMessage(msg.Data)
			if err != nil {
				continue
			}
			c.MessageChan <- smsg
		}
	}()
}

func (c *ChainNotify) subscribeBlock() {

	ctx := c.ctx

	go func() {
		sub, err := c.fa.GetBlkTopic().Subscribe()
		if err != nil {
			log.Error(err)
			return
		}

		waitTime := 8 * time.Second
		ti := time.NewTicker(waitTime + time.Second)
		height := util.NsChainEpoch(0)
		var blks []*util.NsBlockMsg
		var took uint64

		for {
			select {
			case <-ti.C:
				tblks := blks[:]
				blks = []*util.NsBlockMsg{}
				if len(tblks) > 0 {
					c.BlockChan <- tblks
					height = tblks[0].Header.Height
					ti.Reset(time.Duration(uint64(time.Now().Unix())-tblks[0].Header.Timestamp)*time.Second + waitTime)
				}
				took = 0

			default:
				if took > 16 {
					continue
				}
				tctx, cancel := context.WithTimeout(ctx, waitTime)
				msg, err := sub.Next(tctx)
				if err != nil {
					// log.Error(err)
					continue
				}
				bmsg, err := util.NsDecodeBlockMsg(msg.Data)
				if err != nil {
					log.Error(err)
					continue
				}
				log.Debugf("height %d", bmsg.Header.Height)
				if bmsg.Header.Height > height {
					blks = append(blks, bmsg)
					ti.Reset(waitTime + time.Second)
					took = uint64(time.Now().Unix()) - bmsg.Header.Timestamp
				}
				cancel()
			}
		}
	}()
}

func (c *ChainNotify) Publish(ctx context.Context, b []byte) error {
	return c.fa.GetMsgTopic().Publish(ctx, b)
}

func NewChainNotify(fa *P2pLotusAPI) (*ChainNotify, error) {
	ctx := context.TODO()
	c := &ChainNotify{}
	c.ctx = ctx
	c.Cache = gcache.New(65536).LRU().Build()
	c.fa = fa

	c.MessageChan = make(chan *util.NsSignedMessage, 1024)
	c.BlockChan = make(chan []*util.NsBlockMsg, 1024)

	return c, nil
}
