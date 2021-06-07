module gitlab.ns/lotus-worker

go 1.14

replace (
	github.com/filecoin-project/filecoin-ffi => ./extern/lotus/extern/filecoin-ffi
	github.com/filecoin-project/lotus => ./extern/lotus
	github.com/supranational/blst => ./extern/lotus/extern/blst
)

require (
	github.com/docker/go-units v0.4.0
	github.com/fatih/color v1.10.0
	github.com/filecoin-project/filecoin-ffi v0.30.4-0.20200910194244-f640612a1a1f
	github.com/filecoin-project/go-address v0.0.5
	github.com/filecoin-project/go-amt-ipld/v3 v3.0.0
	github.com/filecoin-project/go-bitfield v0.2.4
	github.com/filecoin-project/go-fil-commcid v0.0.0-20201016201715-d41df56b4f6a
	github.com/filecoin-project/go-jsonrpc v0.1.4-0.20210217175800-45ea43ac2bec
	github.com/filecoin-project/go-state-types v0.1.0
	github.com/filecoin-project/lotus v0.0.0-00010101000000-000000000000
	github.com/filecoin-project/specs-actors v0.9.13
	github.com/filecoin-project/specs-actors/v2 v2.3.5-0.20210114162132-5b58b773f4fb
	github.com/filecoin-project/specs-actors/v3 v3.1.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.1.4
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipld-cbor v0.0.5
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-log/v2 v2.1.3
	github.com/jinzhu/gorm v1.9.16
	github.com/libp2p/go-libp2p v0.12.0 // indirect
	github.com/libp2p/go-libp2p-connmgr v0.2.4 // indirect
	github.com/libp2p/go-libp2p-core v0.7.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.1 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.4.2-0.20210212194758-6c1addf493eb
	github.com/multiformats/go-multihash v0.0.14
	github.com/prometheus/common v0.10.0
	github.com/urfave/cli/v2 v2.3.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20210219115102-f37d292932f2
	github.com/whyrusleeping/go-logging v0.0.1 // indirect
	go.opencensus.io v0.22.5
	go.uber.org/fx v1.13.1
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/exp v0.0.0-20210220032938-85be41e4509f // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
)
