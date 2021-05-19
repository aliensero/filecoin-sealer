package up2p

import (
	"context"
	"net"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
	"gitlab.ns/lotus-worker/util"
)

type ChainInfo struct {
	cbs      dtypes.ChainBlockstore
	blocksrv dtypes.ChainBlockService
}

func (ci *ChainInfo) GetBlock(cstr string) (*types.BlockHeader, error) {
	c, err := cid.Parse(cstr)
	if err != nil {
		return nil, err
	}
	var blk *types.BlockHeader
	err = ci.cbs.View(c, func(b []byte) (err error) {
		blk, err = types.DecodeBlock(b)
		return
	})
	return blk, err
}

func (ci *ChainInfo) GetSignedMessage(cstr string) (*types.SignedMessage, error) {
	c, err := cid.Parse(cstr)
	if err != nil {
		return nil, err
	}
	var msg *types.SignedMessage
	err = ci.cbs.View(c, func(b []byte) (err error) {
		msg, err = types.DecodeSignedMessage(b)
		return
	})
	return msg, err
}

func (ci *ChainInfo) GetMessage(cstr string) (*types.Message, error) {
	c, err := cid.Parse(cstr)
	if err != nil {
		return nil, err
	}
	var msg *types.Message
	err = ci.cbs.View(c, func(b []byte) (err error) {
		msg, err = types.DecodeMessage(b)
		return
	})
	return msg, err
}

func (ci *ChainInfo) FetchMessage(cstr string) ([]*types.Message, error) {
	cid, err := util.NsCidDecode(cstr)
	if err != nil {
		return nil, err
	}
	return FetchMsgByCid(context.TODO(), ci.cbs, ci.blocksrv, cid)
}

func (ci *ChainInfo) FetchSigMessage(cstr string) ([]*types.SignedMessage, error) {
	cid, err := util.NsCidDecode(cstr)
	if err != nil {
		return nil, err
	}
	return FetchSigMsgByCid(context.TODO(), ci.cbs, ci.blocksrv, cid)
}

type Faddr string

func ServerRPC(cbs dtypes.ChainBlockstore, blocksrv dtypes.ChainBlockService, addr Faddr) error {
	ci := ChainInfo{
		cbs,
		blocksrv,
	}
	rpcServer := jsonrpc.NewServer()
	mux := mux.NewRouter()
	rpcServer.Register("CHAINSWAP", &ci)
	mux.Handle("/rpc/v0", rpcServer)
	mux.PathPrefix("/").Handler(http.DefaultServeMux)

	srv := &http.Server{
		Handler: mux,
	}
	log.Info("Setting up control endpoint at " + addr)
	nl, err := net.Listen("tcp", string(addr))
	if err != nil {
		return err
	}
	go srv.Serve(nl)
	return nil
}
