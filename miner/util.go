package miner

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc"
	"gitlab.ns/lotus-worker/util"
)

func ConnectWorker(url string) (jsonrpc.ClientCloser, *util.WorkerAPI, error) {
	var wi util.WorkerAPI
	closer, err := jsonrpc.NewMergeClient(context.TODO(), url, "NSWORKER",
		[]interface{}{
			&wi,
		},
		nil,
	)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	log.Warnf("ConnectWorker connect error %v", err)
	return closer, &wi, nil
}
