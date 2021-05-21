package up2p

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/lib/peermgr"
	"github.com/filecoin-project/lotus/node"
	"github.com/filecoin-project/lotus/node/modules"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/jinzhu/gorm"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"gitlab.ns/lotus-worker/util"
	"gitlab.ns/lotus-worker/util/push"
	"go.uber.org/fx"
)

type DbParams struct {
	User     string
	Password string
	Ip       string
	Port     string
	Database string
}

var notifyFIL abi.TokenAmount
var pushOnOff bool
var listenAddrs []string
var listenResetLen = 4
var defaultNotFIL = "10000000000000000000000"
var onlyTran = true
var queryPriceUrl1 = "https://usdapi2.btc126.vip/coingecko.php?from=price&coin=filecoin"
var queryPriceUrl2 = "https://dncapi.bqrank.net/api/v2/Coin/market_ticker?pagesize=1&code=filecoinnew"

func init() {
	var err error
	notifyFIL, err = big.FromString(defaultNotFIL)
	if err != nil {
		panic(err)
	}
	listenAddrs = []string{
		"f1lwxgdpixzhegeedvhgafjcst3wooxnyxre2izty",
		"f1gdjzspoikg5ktff4a3gzf2m7wi3r7hva6ajjkca",
		"f1577vnkythpvyh3mb7knnmckysy6bgygldwsgcya",
		"f1t4xbd6dha6wyzcqwmnovqwt6b24hvz3cdm2mwiy",
	}
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
	node.Override(new(dtypes.ExposedBlockstore), node.From(new(blockstore.Blockstore))),
	node.Override(new(dtypes.ChainBitswap), modules.ChainBitswap),
	node.Override(new(DbParams), func() DbParams {
		return DbParams{
			User:     "root",
			Password: "w123456W",
			Ip:       "127.0.0.1",
			Port:     "3306",
			Database: "db_worker",
		}
	}),
	node.Override(new(Faddr), func() Faddr {
		return Faddr("127.0.0.1:4321")
	}),
	node.Override(node.SetGenesisKey, func(addr Faddr) {
		apiHandle := apiHandle{}
		err := ServerFrom("COLCONTROL", addr, &apiHandle, handleFuncMap)
		if err != nil {
			panic(err)
		}
	}),
	node.Override(node.HandleIncomingBlocksKey, HandleCollectTran),
)

func HandleCollectTran(mctx helpers.MetricsCtx, lc fx.Lifecycle, dbparams DbParams, ps *pubsub.PubSub, h host.Host, nn dtypes.NetworkName, bs blockstore.Blockstore, rem dtypes.ChainBitswap) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	topic := build.MessagesTopic(nn)
	if err := ps.RegisterTopicValidator(topic, checkIncomingMessage(h)); err != nil {
		panic(err)
	}

	log.Infof("subscribing to pubsub topic %s", topic)

	blocksub, err := ps.Subscribe(topic) //nolint
	if err != nil {
		panic(err)
	}

	if dbparams.Password == "" {
		dbparams.Password = "w123456W"
	}

	if dbparams.Port == "" {
		dbparams.Port = "13306"
	}

	db, err := util.InitMysql(dbparams.User, dbparams.Password, dbparams.Ip, dbparams.Port, dbparams.Database)
	if err != nil {
		panic(err)
	}
	sum30s := big.NewInt(0)
	cmpValue, err := big.FromString(defaultNotFIL)
	if err != nil {
		panic(err)
	}
	time30s := time.NewTicker(30 * time.Second)
	defer time30s.Stop()
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

		smsg, ok := msg.ValidatorData.(*types.SignedMessage)
		if !ok {
			log.Warnf("pubsub block validator passed on wrong type: %#v", msg.ValidatorData)
			continue
		}

		if smsg.Message.Method != 0 && onlyTran {
			continue
		}
		select {
		case <-time30s.C:
			msgType := "transacaion sum-" + types.FIL(sum30s).String()
			log.Infof("intervar message %v push?%v", msgType, sum30s.GreaterThanEqual(cmpValue))
			if sum30s.GreaterThanEqual(cmpValue) {
				pushMessage("", nil, msgType)
			}
			sum30s = big.NewInt(0)
		default:
			sum30s.Int = sum30s.Add(sum30s.Int, smsg.Message.Value.Int)
		}

		cid := smsg.Cid().String()
		go handleMessage(cid, &smsg.Message, db, &notifyFIL)
	}
}

func handleMessage(cid string, mp *types.Message, db *gorm.DB, maxFIL *abi.TokenAmount) {
	m := *mp
	err := db.Where("cid=?", cid).Find(&util.DbTransaction{}).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			dr := util.DbTransaction{
				Cid:     cid,
				Acct:    m.From.String(),
				Type:    util.DR,
				Value:   m.Value.String(),
				Method:  uint64(m.Method),
				NetName: NATNAME,
			}
			err := db.Create(&dr).Error
			if err != nil {
				log.Errorf("db create dr %v error %v", dr, err)
			}
			cr := util.DbTransaction{
				Cid:     cid,
				Acct:    m.To.String(),
				Type:    util.CR,
				Value:   m.Value.String(),
				Method:  uint64(m.Method),
				NetName: NATNAME,
			}
			err = db.Create(&cr).Error
			if err != nil {
				log.Errorf("db create cr %v error %v", dr, err)
			}
			if m.Value.GreaterThan(*maxFIL) {
				go pushMessage(cid, &m, "higher price")
			}
			for _, la := range listenAddrs {
				if m.From.String() == la {
					go pushMessage(cid, &m, la+"-seal")
				}
				if m.To.String() == la {
					go pushMessage(cid, &m, la+"-buy")
				}
			}
		} else {
			log.Errorf("db find %v error %v", cid, err)
		}
	}
}

func pushMessage(cid string, m *types.Message, msgType string) {
	if (m != nil && m.Method != 0) || pushOnOff {
		return
	}
	ps := push.PostMessage{}
	cs := ""
	from := ""
	value := ""
	if m != nil {
		content, err := m.MarshalJSON()
		if err != nil {
			log.Errorf("Marshal Message %v error %V", m, err)
			return
		}
		cs = string(content)
		from = m.From.String()
		value = types.FIL(m.Value).String()
	}

	price := "$0"
	resp, err := http.Get(queryPriceUrl2)
	if err == nil {
		defer func() {
			if resp.Body != nil {
				resp.Body.Close()
			}
		}()
		pbuf, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			log.Infof("http request price %v", string(pbuf))
			pbss := strings.Split(string(pbuf), `"price":`)
			if len(pbss) > 1 {
				prs := strings.Split(pbss[1], ",")
				if len(prs) > 0 {
					price = prs[0]
				}
			}
		} else {
			log.Errorf("Read price error %v", err)
		}
	}

	ps.Content = cs
	ps.Summary = msgType + "-" + price + "-" + value
	ps.Url = "https://filfox.info/zh/address/" + from
	if len(ps.Summary) > 100 {
		ps.Summary = ps.Summary[:95] + " FIL"
	}
	if ps.Content == "" {
		ps.Content = ps.Summary
	}
	err = push.WxPush(ps)
	if err != nil {
		log.Errorf("Push Messasge %v error %v", ps, err)
	}
}

func getMemBlockServiceSession(ctx context.Context, bs blockstore.Blockstore, rem dtypes.ChainBitswap) *bserv.Session {
	bsservice := bserv.New(bs, rem)
	return bserv.NewSession(ctx, bsservice)
}

type apiHandle struct{}

var handleFuncMap = map[string]func(http.ResponseWriter,
	*http.Request){
	"setfil": func(resp http.ResponseWriter, req *http.Request) {
		var err error
		keys := req.URL.Query()["fil"]
		if len(keys) == 0 {
			resp.Write([]byte("?fil=50000000000000000000"))
			return
		}
		notifyFIL, err = big.FromString(keys[0])
		if err != nil {
			resp.Write([]byte(fmt.Sprintf("no notify FIL error %v", err)))
			return
		}
		resp.Write([]byte(keys[0]))
	},
	"appuid": func(resp http.ResponseWriter, req *http.Request) {
		keys := req.URL.Query()["uid"]
		if len(keys) == 0 {
			resp.Write([]byte("?uid=UID_xxxx"))
			return
		}
		push.Uids = append(push.Uids, keys[0])
		resp.Write([]byte(keys[0]))
	},
	"resetuids": func(resp http.ResponseWriter, req *http.Request) {
		push.Uids = push.Uids[:1]
		ret := ""
		for _, u := range push.Uids {
			ret += u + ","
		}
		resp.Write([]byte(ret))
	},
	"displayuids": func(resp http.ResponseWriter, req *http.Request) {
		ret := ""
		for _, u := range push.Uids {
			ret += u + ","
		}
		resp.Write([]byte(ret))
	},
	"appaddr": func(resp http.ResponseWriter, req *http.Request) {
		keys := req.URL.Query()["addr"]
		if len(keys) == 0 {
			resp.Write([]byte("?addr=xxxx"))
			return
		}
		listenAddrs = append(listenAddrs, keys[0])
		resp.Write([]byte(keys[0]))
	},
	"resetaddrs": func(resp http.ResponseWriter, req *http.Request) {
		listenAddrs = listenAddrs[:listenResetLen]
		ret := ""
		for _, l := range listenAddrs {
			ret += l + ","
		}
		resp.Write([]byte(ret))
	},
	"displayaddrs": func(resp http.ResponseWriter, req *http.Request) {
		ret := ""
		for _, l := range listenAddrs {
			ret += l + ","
		}
		resp.Write([]byte(ret))
	},
}

func (ah *apiHandle) ChanNotifyFIL(nf string) (err error) {
	notifyFIL, err = big.FromString(nf)
	return
}

func (ah *apiHandle) PushOnOff() (bool, error) {
	pushOnOff = !pushOnOff
	return pushOnOff, nil
}

func (ah *apiHandle) AppendNotifyAddr(addr string) (string, error) {
	listenAddrs = append(listenAddrs, addr)
	return addr, nil
}

func (ah *apiHandle) ClearNotifyAddr(addr string) error {
	listenAddrs = listenAddrs[:listenResetLen]
	return nil
}

func (ah *apiHandle) DisplayNotifyAddr() ([]string, error) {
	return listenAddrs, nil
}

func (ah *apiHandle) AppendUids(uid string) (string, error) {
	push.Uids = append(push.Uids, uid)
	return uid, nil
}

func (ah *apiHandle) ClearUids() error {
	push.Uids = push.Uids[:1]
	return nil
}

func (ah *apiHandle) DisplayUids() ([]string, error) {
	return push.Uids, nil
}
