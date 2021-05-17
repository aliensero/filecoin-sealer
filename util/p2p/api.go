package up2p

import (
	"net"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
)

type ChainInfo struct {
	cbs dtypes.ChainBlockstore
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

type Faddr string

func ServerRPC(cbs dtypes.ChainBlockstore, addr Faddr) error {
	ci := ChainInfo{
		cbs,
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
