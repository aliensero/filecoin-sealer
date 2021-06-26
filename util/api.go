package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-jsonrpc"
)

func ConnMiner(minerUrl string) (jsonrpc.ClientCloser, *MinerAPI, error) {

	var mi MinerAPI
	closer, err := jsonrpc.NewMergeClient(context.TODO(), minerUrl, "NSMINER",
		[]interface{}{
			&mi,
		},
		nil,
		jsonrpc.WithNoReconnect(),
	)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	log.Infof("lotusMiner connect error %v", err)
	return closer, &mi, nil
}

var QueryUndefindType = "QueryUndefindType"

type QuryCode int

const (
	Ok QuryCode = iota
	Err
)

type QueryTaskInfoResult struct {
	ResultCode QuryCode
	Err        string
	Results    []DbTaskInfo
}

func (qr *QueryTaskInfoResult) ToString() string {
	buf, err := json.Marshal(qr)
	if err != nil {
		return fmt.Sprintf("Marshal %v error %v", qr, err)
	}
	return string(buf)
}

type QueryPostInfoResult struct {
	ResultCode QuryCode
	Err        string
	Results    []DbPostInfo
}

func (qr *QueryPostInfoResult) ToString() string {
	buf, err := json.Marshal(qr)
	if err != nil {
		return fmt.Sprintf("Marshal %v error %v", qr, err)
	}
	return string(buf)
}

type MinerAPI struct {
	CheckServer         func() bool
	QueryOnly           func(DbTaskInfo) (QueryTaskInfoResult, error)
	QueryTask           func(RequestInfo) (QueryTaskInfoResult, error)
	GetTicket           func(actorID int64, sectorNum int64) ([]byte, error)
	RecieveTaskResult   func(actorID int64, sectorNum int64, taskType string, reqID string, isErr bool, tr TaskResult) error
	QueryLastResult     func(actorID int64, sectorNum int64, lastReqID string) (DbTaskLog, error)
	UpdateTask          func(actorID int64, sectorNum int64, taskType string, state int64) (DbTaskInfo, error)
	RetryTask           func(RequestInfo) (QueryTaskInfoResult, error)
	QueryRetry          func(RequestInfo) (QueryTaskInfoResult, error)
	ClearProving        func() error
	UpdateTaskLog       func(reqInfo RequestInfo, result string) (string, error)
	WorkerLogin         func(string, string, string) (string, error)
	QueryToPoSt         func(reqInfo RequestInfo) (QueryPostInfoResult, error)
	StartSubmitPoSt     func(trnmsg PoStTransfer) string
	AbortPoSt           func(actorID int64) (string, error)
	QueryMinerPoStInfo  func(actorID int64) (MinerPoStInfo, error)
	TestStartSubmitPoSt func(trnmsg PoStTransfer) (int, error)

	AddToServer      func(actorID int64, privatkey string) error
	DeleteFromServer func(actorID int64) error
	DisplayServer    func() (interface{}, error)

	GetSeedRand              func(actorID int64, sectorNum int64) (string, error)
	SendPreCommitByPrivatKey func(prihex string, actorID int64, sectorNum int64, deposit string) (string, error)
	SendCommitByPrivatKey    func(prihex string, actorID int64, sectorNum int64, deposit string) (string, error)
	MinerInfo                func(actorID uint64) (string, error)
	TxByPrivatKey            func(prihex string, to string, amt string) (string, error)
	AddTask                  func(actorID int64, sectorNum int64, taskType string, minerPaht string, unsealedPath string, sealerProof string, proofType int, pieceCIDstr string) (int64, error)
	GenerateAddress          func(addrType NsKeyType) (string, error)
	CreateMiner              func(prihex, owner, worker string, sealProofType int64) (string, error)
	MpoolPending             func() ([]*NsSignedMessage, error)
	GasEstimateMessageGas    func(msg *NsMessage) (*NsMessage, error)
	MpoolPush                func(sm *NsSignedMessage) (NsCid, error)
	SignedMessage            func(prikey string, msg NsMessage) (*NsSignedMessage, error)

	CheckRecoveries      func(actorID int64, addrInfo string, addrType string, dlIdx uint64) (string, error)
	ResetSyncPoStMap     func() string
	ReFindPoStTable      func(actorID int64) ([]int64, error)
	AddPoStActor         func(actorID int64, addrinfo string, addrtype string) (string, error)
	WinPoStProofs        func(minerID int64, prihex string) (NsDeadLineInfo, error)
	TestWindowPostProofs func(minerID uint64,
		sectorInfoPath string, path string, proofTyep int64, challange int64, deadlineIndex int64, partInx int, prihex string) (int, error)
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

type LotusApiStr string

func NewPubLotusApi1(addr LotusApiStr) (LotusAPI, jsonrpc.ClientCloser, error) {
	return NsNewFullNodeRPC(context.TODO(), string(addr), nil)
}

type MsgLookup struct {
	Message   NsCid // Can be different than requested, in case it was replaced, but only gas values changed
	Receipt   NsMessageReceipt
	ReturnDec interface{}
	TipSet    NsTipSetKey
	Height    NsChainEpoch
}

type ChildProcessInfo struct {
	ActorID   int64
	SectorNum int64
	Pid       int
}

func (cp *ChildProcessInfo) ToString() string {
	buf, err := json.Marshal(cp)
	if err != nil {
		return fmt.Sprintf("Marshal %v error %v", cp, err)
	}
	return string(buf)
}

func ConnectWorker(url string) (jsonrpc.ClientCloser, *WorkerAPI, error) {
	var wi WorkerAPI
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

type WorkerAPI struct {
	AddToServer      func(actorID int64, tasks []string) error
	DeleteFromServer func(actorID int64) error
	DisplayServer    func(actorID int64) (interface{}, error)

	ProcessPrePhase1    func(actorID int64, sectorNum int64, binPath string) (ChildProcessInfo, error)
	ProcessPrePhase2    func(actorID int64, sectorNum int64, binPath string) (ChildProcessInfo, error)
	ProcessCommitPhase1 func(actorID int64, sectorNum int64, binPath string) (ChildProcessInfo, error)
	ProcessCommitPhase2 func(actorID int64, sectorNum int64, binPath string) (ChildProcessInfo, error)
	RetryTaskPID        func(actorID int64, sectorNum int64, taskType string, binPath string) (string, error)
	CheckSession        func(string) bool

	AddPoStActor     func(actorID int64, addrinfo string, addrtype string) (string, error)
	DisplayPoStActor func() (string, error)
	RetryWinPoSt     func(actorID int64, addrInfo string, addrTyep string) (string, error)
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

type MinerPoStInfo struct {
	Di         NsDeadLineInfo
	Partitions []NsPartition
	Rand       NsPoStRandomness
}
