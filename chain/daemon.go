package chain

import (
	"context"
	"net"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/gorilla/mux"
	"gitlab.ns/lotus-worker/util"
)

func Daemon(lotusapi string, listen string) error {

	lotusApi, close, err := util.NewPubLotusApi(lotusapi)
	if err != nil {
		return err
	}
	defer close()
	p2papi, err := NewP2pLotusAPI(lotusApi)
	if err != nil {
		return err
	}
	defer p2papi.Close()

	rpcServer := jsonrpc.NewServer()
	mux := mux.NewRouter()
	rpcServer.Register("P2PAPI", p2papi)
	mux.Handle("/rpc/v0", rpcServer)
	mux.PathPrefix("/").Handler(http.DefaultServeMux)

	srv := &http.Server{
		Handler: mux,
	}
	log.Info("Setting up control endpoint at " + listen)
	nl, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	return srv.Serve(nl)
}

func NewChainRPC(ctx context.Context, addr string) (*P2pAPI, jsonrpc.ClientCloser, error) {
	var res P2pAPI
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "P2PAPI",
		[]interface{}{
			&res,
		}, nil)
	return &res, closer, err
}
