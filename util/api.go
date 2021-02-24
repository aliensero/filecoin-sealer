package util

import (
	"context"
	"net/http"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-jsonrpc"
)

type MinerAPI struct {
	CheckServer       func() bool
	AddTask           func(actorID int64, taskType string, minerPaht string, unsealedPath string) (int64, error)
	QueryOnly         func(DbTaskInfo) ([]DbTaskInfo, error)
	QueryTask         func(RequestInfo) (DbTaskInfo, error)
	GetTicket         func(actorID int64, sectorNum int64) ([]byte, error)
	RecieveTaskResult func(actorID int64, sectorNum int64, taskType string, reqID string, isErr bool, result []byte) error
	QueryLastResult   func(actorID int64, sectorNum int64, lastReqID string) (DbTaskLog, error)
	UpdateTask        func(actorID int64, sectorNum int64, taskType string, state int64) (DbTaskInfo, error)
	RetryTask         func(RequestInfo) (DbTaskInfo, error)
	QueryRetry        func(RequestInfo) (DbTaskInfo, error)
	ClearProving      func() error
	UpdateTaskLog     func(reqInfo RequestInfo, result string) (string, error)
	WorkerLogin       func(string, string, string) (string, error)
	QueryToPoSt       func(reqInfo RequestInfo) ([]DbPostInfo, error)
	StartSubmitPoSt   func(trnmsg PoStTransfer) string
	ResetSyncPoStMap  func() string
	ReFindPoStTable   func(actorID int64) ([]int64, error)
	AbortPoSt         func(actorID int64) (string, error)
}

func authHeader(token string) http.Header {
	log.Warnf("API Token %s", token)
	if token != "" {
		headers := http.Header{}
		headers.Add("Authorization", "Bearer "+token)
		return headers
	}
	return nil
}

func NewLotusApi(addr string, token string) (LotusAPI, jsonrpc.ClientCloser, error) {
	return NsNewFullNodeRPC(context.TODO(), addr, authHeader(token))
}

func NewPubLotusApi(addr string) (LotusAPI, jsonrpc.ClientCloser, error) {
	return NsNewFullNodeRPC(context.TODO(), addr, nil)
}

type MsgLookup struct {
	Message   NsCid // Can be different than requested, in case it was replaced, but only gas values changed
	Receipt   NsMessageReceipt
	ReturnDec interface{}
	TipSet    NsTipSetKey
	Height    NsChainEpoch
}

type WorkerAPI struct {
	CheckSession func(string) bool
}

type VanillaProof struct {
	SectorNum uint64
	Proof     []byte
}

type PoStTransfer struct {
	WorkerID         string
	HostName         string
	PoStProofType    NsRegisteredPoStProof
	Actor            NsAddress
	AddrInfo         string
	AddrType         string
	MinerID          uint64
	CurrentEpoch     int64
	CloseEpoch       int64
	Challenge        int64
	Dealine          uint64
	PartitionIndex   int
	AllProveBitfield bitfield.BitField
	ToProveBitfield  bitfield.BitField
	FaultBitfield    bitfield.BitField
	Proofs           []VanillaProof
	Randomness       NsPoStRandomness
	Err              string
}
