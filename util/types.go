package util

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/exitcode"

	p2pcrypt "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/filecoin-ffi/generated"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	"github.com/filecoin-project/lotus/chain/gen"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/lotus/lib/sigs"
	_ "github.com/filecoin-project/lotus/lib/sigs/bls"
	_ "github.com/filecoin-project/lotus/lib/sigs/secp"
	lminer "github.com/filecoin-project/lotus/miner"

	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	proof0 "github.com/filecoin-project/specs-actors/actors/runtime/proof"
	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"

	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/filecoin-project/lotus/node/modules/lp2p"
	blockadt "github.com/filecoin-project/specs-actors/actors/util/adt"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	miner2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	power2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/power"
	block "github.com/ipfs/go-block-format"
	cbor "github.com/ipfs/go-ipld-cbor"
	mh "github.com/multiformats/go-multihash"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var NsSealProofType_512MiB = 7
var NsSealProofTyep_32GiB = 8

type NsSectorNum = abi.SectorNumber
type NsActorID = abi.ActorID
type NsSealRandomness = abi.SealRandomness
type NsRegisteredSealProof = abi.RegisteredSealProof
type NsSeed = abi.InteractiveSealRandomness
type NsRandomness = abi.Randomness
type NsChainEpoch = abi.ChainEpoch
type NsTokenAmount = abi.TokenAmount
type NsMethodNum = abi.MethodNum
type NsPaddedPieceSize = abi.PaddedPieceSize
type NsRegisteredPoStProof = abi.RegisteredPoStProof
type NsPoStRandomness = abi.PoStRandomness

type NsCid = cid.Cid

var Parse = cid.Parse
var NsCidDecode = cid.Decode

type NsPieceInfo = abi.PieceInfo

func NewNsPieceInfo(v string, sectorSizeInt int64) ([]NsPieceInfo, error) {

	npis := make([]NsPieceInfo, 0, 1)
	vCid, err := cid.Decode(v)
	if err != nil {
		return []NsPieceInfo{}, err
	}
	pi := abi.PieceInfo{
		Size:     abi.PaddedPieceSize(sectorSizeInt),
		PieceCID: vCid,
	}
	npis = append(npis, pi)
	return npis, nil
}

type NsTipSet = types.TipSet
type NsTipSetKey = types.TipSetKey
type NsMessageReceipt = types.MessageReceipt
type NsMessage = types.Message
type NsSignedMessage = types.SignedMessage
type NsFIL = types.FIL
type NsKeyType = types.KeyType
type NsKeyInfo = types.KeyInfo
type NsActor = types.Actor
type NsBlockHeader = types.BlockHeader
type NsBlockMsg = types.BlockMsg
type NsElectionProof = types.ElectionProof
type NsBeaconEntry = types.BeaconEntry
type NsTicket = types.Ticket
type NsFullBlock = types.FullBlock
type NsMsgMeta = types.MsgMeta

var ParseFIL = types.ParseFIL
var NsNewInt = types.NewInt
var NsNewTipSet = types.NewTipSet
var NsDecodeBlockMsg = types.DecodeBlockMsg
var NsBigFromBytes = types.BigFromBytes

type NsNetworkName = dtypes.NetworkName

type NsSectorPreCommitInfo = miner0.SectorPreCommitInfo
type NsProveCommitSectorParams = miner0.ProveCommitSectorParams
type NsMinerInfo = miner0.MinerInfo
type NsSubmitWindowedPoStParams = miner0.SubmitWindowedPoStParams
type NsPoStPartition = miner0.PoStPartition
type NsDeclareFaultsRecoveredParams = miner0.DeclareFaultsRecoveredParams
type NsRecoveryDeclaration = miner0.RecoveryDeclaration

var NsMethods = builtin2.MethodsMiner
var NsMarketAddr = builtin2.StorageMarketActorAddr

type NsDomainSeparationTag = crypto.DomainSeparationTag
type NsSigType = crypto.SigType
type NsSignature = crypto.Signature

var NsSigTypeBLS = crypto.SigTypeBLS

var NsDomainSeparationTag_ElectionProofProduction = crypto.DomainSeparationTag_ElectionProofProduction
var NsDomainSeparationTag_TicketProduction = crypto.DomainSeparationTag_TicketProduction

var NsDomainSeparationTag_PoStChainCommit = crypto.DomainSeparationTag_PoStChainCommit
var NsDomainSeparationTag_InteractiveSealChallengeSeed = crypto.DomainSeparationTag_InteractiveSealChallengeSeed
var NsDomainSeparationTag_SealRandomness = crypto.DomainSeparationTag_SealRandomness
var NsDomainSeparationTag_WindowedPoStChallengeSeed = crypto.DomainSeparationTag_WindowedPoStChallengeSeed

type NsAddress = address.Address

var NsNewFromString = address.NewFromString
var NsNewIDAddress = address.NewIDAddress
var NsNewSecp256k1Address = address.NewSecp256k1Address
var NsNewBLSAddress = address.NewBLSAddress
var NsIDFromAddress = address.IDFromAddress

var NsexitcodeOk = exitcode.Ok
var NsexitcodeSysErrInsufficientFunds = exitcode.SysErrInsufficientFunds
var NsexitcodeSysErrOutOfGas = exitcode.SysErrOutOfGas

type Nsbig = big.Int

var NsSealRandomnessLookback = policy.SealRandomnessLookback

type NsFilRegisteredSealProof = generated.FilRegisteredSealProof
type NsRawString = generated.RawString

var NsFilGeneratePieceCommitment = generated.FilGeneratePieceCommitment
var NsFilDestroyGeneratePieceCommitmentResponse = generated.FilDestroyGeneratePieceCommitmentResponse
var NsFCPResponseStatusFCPNoError = generated.FCPResponseStatusFCPNoError

var NsPieceCommitmentV1ToCID = commcid.PieceCommitmentV1ToCID
var NsCIDToPieceCommitmentV1 = commcid.CIDToPieceCommitmentV1

var NsSigsToPublic = sigs.ToPublic

var NsactSerializeParams = actors.SerializeParams

type NsWithdrawBalanceParams = miner2.WithdrawBalanceParams

type NsSectorOnChainInfo = miner.SectorOnChainInfo
type NsSectorPreCommitOnChainInfo = miner.SectorPreCommitOnChainInfo
type NsSectorLocation = miner.SectorLocation
type NsDeadline = miner.Deadline
type NsMinerPartition = miner.Partition

var NsWithdrawBalance = miner.Methods.WithdrawBalance
var NsMinerLoad = miner.Load
var NsWinningPoStProofTypeFromWindowPoStProofType = miner.WinningPoStProofTypeFromWindowPoStProofType

type NsMiningBase = lminer.MiningBase

type NsMessageSendSpec = api.MessageSendSpec
type LotusAPI = api.FullNode
type NsPartition = api.Partition
type NsMiningBaseInfo = api.MiningBaseInfo
type NsBlockTemplate = api.BlockTemplate

var NsNewAPIBlockstore = blockstore.NewAPIBlockstore
var NsNewMemory = blockstore.NewMemory

var NsCurrentNetwork = address.CurrentNetwork
var NsMainNet = address.Mainnet

var NsNewFullNodeRPC = client.NewFullNodeRPC

type NsDeadLineInfo = dline.Info

type NsPrivateSectorInfo = ffi.PrivateSectorInfo
type NsFallbackChallenges = ffi.FallbackChallenges

var NsNewSortedPrivateSectorInfo = ffi.NewSortedPrivateSectorInfo
var NsGenerateWindowPoSt = ffi.GenerateWindowPoSt
var NsGeneratePoStFallbackSectorChallenges = ffi.GeneratePoStFallbackSectorChallenges
var NsGenerateSingleVanillaProof = ffi.GenerateSingleVanillaProof
var NsGenerateWindowPoStWithVanilla = ffi.GenerateWindowPoStWithVanilla

type NsSectorInfo = proof0.SectorInfo
type NsPoStProof = proof0.PoStProof

var NsActorStore = store.ActorStore
var NsDrawRandomness = store.DrawRandomness

var NsDhtProtocolName = build.DhtProtocolName
var NsBlocksTopic = build.BlocksTopic
var NsBuiltinBootstrap = build.BuiltinBootstrap
var NsTicketRandomnessLookback = build.TicketRandomnessLookback
var NsBlockDelaySecs = build.BlockDelaySecs

type NsLibp2pOpts = lp2p.Libp2pOpts
type NsRawHost = lp2p.RawHost
type NsBaseIpfsRouting = lp2p.BaseIpfsRouting

var NsDefaultTransports = lp2p.DefaultTransports
var NsAddrsFactory = lp2p.AddrsFactory
var NsSmuxTransport = lp2p.SmuxTransport
var NsNoRelay = lp2p.NoRelay
var NsSecurity = lp2p.Security
var NsRoutedHost = lp2p.RoutedHost

type NsSignFunc = gen.SignFunc

var NsComputeVRF = gen.ComputeVRF

var NsGetFullNodeAPI = cli.GetFullNodeAPI

var NsNewCborStore = cbor.NewCborStore

func NsSealPreCommitPhase1(nsProof NsRegisteredSealProof, cacheDirPath, stagedSectorPath, sealedSectorPath string, nsSectorNum NsSectorNum, nsActorID NsActorID, nsTicket NsSealRandomness, nsPieceInfo []NsPieceInfo) ([]byte, error) {

	return ffi.SealPreCommitPhase1(abi.RegisteredSealProof(nsProof), cacheDirPath, stagedSectorPath, sealedSectorPath, abi.SectorNumber(nsSectorNum), abi.ActorID(nsActorID), abi.SealRandomness(nsTicket), nsPieceInfo)
}

func NsSealPreCommitPhase2(phase1Output []byte, cacheDirPath, sealedSectorPath string) (NsCid, NsCid, error) {
	sealedCID, unsealedCID, err := ffi.SealPreCommitPhase2(phase1Output, cacheDirPath, sealedSectorPath)
	return NsCid(sealedCID), NsCid(unsealedCID), err
}

func NsSealCommitPhase1(nsProofType NsRegisteredSealProof, nsSealedCID NsCid, nsUnsealedCID NsCid, cacheDirPath, sealedSectorPath string, nsSectorNum NsSectorNum, nsActorID NsActorID, ticketBytes NsSealRandomness, nsSeed NsSeed, nsPieceInfo []NsPieceInfo) ([]byte, error) {
	return ffi.SealCommitPhase1(abi.RegisteredSealProof(nsProofType), cid.Cid(nsSealedCID), cid.Cid(nsUnsealedCID), cacheDirPath, sealedSectorPath, abi.SectorNumber(nsSectorNum), abi.ActorID(nsActorID), abi.SealRandomness(ticketBytes), abi.InteractiveSealRandomness(nsSeed), nsPieceInfo)
}

func NsSealCommitPhase2(phase1Output []byte, nsSectorNum NsSectorNum, nsActorID NsActorID) ([]byte, error) {
	return ffi.SealCommitPhase2(phase1Output, abi.SectorNumber(nsSectorNum), abi.ActorID(nsActorID))
}

func GenerateKeyByHexString(pkstr string) (*Key, error) {

	pk, err := hex.DecodeString(pkstr)
	if err != nil {
		return nil, err
	}
	var ki NsKeyInfo
	err = json.Unmarshal(pk, &ki)
	if err != nil {
		return nil, err
	}
	return NewKey(ki)
}

func GenerateKey(typ NsKeyType) (*Key, error) {
	ctyp := ActSigType(typ)
	if ctyp == crypto.SigTypeUnknown {
		return nil, xerrors.Errorf("unknown sig type: %s", typ)
	}
	pk, err := sigs.Generate(ctyp)
	if err != nil {
		return nil, err
	}
	ki := NsKeyInfo{
		Type:       typ,
		PrivateKey: pk,
	}
	return NewKey(ki)
}

func GeneratePriKeyHex(typ NsKeyType) (string, error) {
	ctyp := ActSigType(typ)
	if ctyp == crypto.SigTypeUnknown {
		return "", xerrors.Errorf("unknown sig type: %s", typ)
	}
	pk, err := sigs.Generate(ctyp)
	if err != nil {
		return "", err
	}
	ki := NsKeyInfo{
		Type:       typ,
		PrivateKey: pk,
	}
	kbs, err := json.Marshal(ki)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(kbs), nil
}

func GenerateAddrByHexPri(prihex string) (NsAddress, error) {
	ki, err := GenerateKeyByHexString(prihex)
	if err != nil {
		return NsAddress{}, err
	}
	return ki.Address, nil
}

func ActSigType(typ NsKeyType) NsSigType {
	switch typ {
	case types.KTBLS:
		return crypto.SigTypeBLS
	case types.KTSecp256k1:
		return crypto.SigTypeSecp256k1
	default:
		return crypto.SigTypeUnknown
	}
}

type Key struct {
	NsKeyInfo

	PublicKey []byte
	Address   NsAddress
}

func NewKey(keyinfo NsKeyInfo) (*Key, error) {
	k := &Key{
		NsKeyInfo: keyinfo,
	}

	var err error
	k.PublicKey, err = sigs.ToPublic(ActSigType(k.Type), k.PrivateKey)
	if err != nil {
		return nil, err
	}

	switch k.Type {
	case types.KTSecp256k1:
		k.Address, err = NsNewSecp256k1Address(k.PublicKey)
		if err != nil {
			return nil, xerrors.Errorf("converting Secp256k1 to address: %w", err)
		}
	case types.KTBLS:
		k.Address, err = NsNewBLSAddress(k.PublicKey)
		if err != nil {
			return nil, xerrors.Errorf("converting BLS to address: %w", err)
		}
	default:
		return nil, xerrors.Errorf("unsupported key type: %s", k.Type)
	}
	return k, nil

}

func SignMsg(ki NsKeyInfo, msgCid []byte) (*NsSignature, error) {
	return sigs.Sign(ActSigType(ki.Type), ki.PrivateKey, msgCid)
}

func SignMsgByHexPri(prihex string, msghex string) (string, error) {

	msg, err := hex.DecodeString(msghex)
	if err != nil {
		return "", err
	}

	ki, err := GenerateKeyByHexString(prihex)
	if err != nil {
		return "", err
	}
	sigture, err := SignMsg(ki.NsKeyInfo, msg)
	if err != nil {
		return "", err
	}
	sigbytes, err := sigture.MarshalBinary()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sigbytes), nil
}

func VerifyMsg(sig NsSignature, k NsAddress, msgCid []byte) error {
	return sigs.Verify(&sig, k, msgCid)
}

func VerifyMsgByAddr(addrstr string, msghex string, sighex string) error {
	addr, err := address.NewFromString(addrstr)
	if err != nil {
		return err
	}

	msg, err := hex.DecodeString(msghex)
	if err != nil {
		return err
	}

	sigBytes, err := hex.DecodeString(sighex)
	if err != nil {
		return err
	}

	var sig crypto.Signature
	if err := sig.UnmarshalBinary(sigBytes); err != nil {
		return err
	}
	return VerifyMsg(sig, addr, msg)
}

func GenerateCreateMinerSigMsg(lotusApi LotusAPI, prihex, owner, worker string, sealProofType int64, nonce uint64) (*NsSignedMessage, error) {

	addrowner, err := address.NewFromString(owner)
	if err != nil {
		return nil, err
	}
	addrworker, err := address.NewFromString(worker)
	if err != nil {
		return nil, err
	}

	pk, _, err := p2pcrypt.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	peerid, err := peer.IDFromPrivateKey(pk)
	if err != nil {
		return nil, xerrors.Errorf("peer ID from private key: %w", err)
	}

	params, err := actors.SerializeParams(&power2.CreateMinerParams{
		Owner:         addrowner,
		Worker:        addrworker,
		SealProofType: NsRegisteredSealProof(sealProofType),
		Peer:          []byte(peerid),
	})
	if err != nil {
		return nil, err
	}
	msg := NsMessage{
		To:     builtin2.StoragePowerActorAddr,
		From:   addrowner,
		Value:  types.NewInt(0),
		Method: 2,
		Nonce:  nonce,
		Params: params,
	}

	msgptr, err := lotusApi.GasEstimateMessageGas(context.TODO(), &msg, nil, NsTipSetKey{})
	if err != nil {
		log.Errorf("sendMsgByPrivatKey GasEstimateMessageGas error %v", err)
		return nil, err
	}
	msg = *msgptr
	return GenerateUtilSigMsg(lotusApi, prihex, msg)
}

func GenerateUtilSigMsg(lotusApi LotusAPI, prihex string, msgParam NsMessage) (*NsSignedMessage, error) {

	cp := msgParam
	msg := &cp
	var err error
	key, err := GenerateKeyByHexString(prihex)
	if err != nil {
		return nil, err
	}
	sigture, err := SignMsg(key.NsKeyInfo, msg.Cid().Bytes())
	if err != nil {
		return nil, err
	}
	sigMsg := NsSignedMessage{
		Message:   *msg,
		Signature: *sigture,
	}
	return &sigMsg, nil
}

func GenerateCreateMinerSigMsgWithDefaultFee(prihex, owner, worker string, sealProofType int64, nonce uint64) (*NsSignedMessage, error) {

	addrowner, err := address.NewFromString(owner)
	if err != nil {
		return nil, err
	}
	addrworker, err := address.NewFromString(worker)
	if err != nil {
		return nil, err
	}

	pk, _, err := p2pcrypt.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	peerid, err := peer.IDFromPrivateKey(pk)
	if err != nil {
		return nil, xerrors.Errorf("peer ID from private key: %w", err)
	}

	params, err := actors.SerializeParams(&power2.CreateMinerParams{
		Owner:         addrowner,
		Worker:        addrworker,
		SealProofType: NsRegisteredSealProof(sealProofType),
		Peer:          []byte(peerid),
	})
	if err != nil {
		return nil, err
	}
	msg := NsMessage{
		To:     builtin2.StoragePowerActorAddr,
		From:   addrowner,
		Value:  types.NewInt(0),
		Method: 2,
		Nonce:  nonce,
		Params: params,
	}

	return GenerateUtilSigMsgWithDefaultFee(prihex, msg)
}

func GenerateCreateMinerSigMsgByAddress(prihex string, addrowner, addrworker NsAddress, sealProofType int64, nonce uint64) (*NsSignedMessage, error) {

	pk, _, err := p2pcrypt.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	peerid, err := peer.IDFromPrivateKey(pk)
	if err != nil {
		return nil, xerrors.Errorf("peer ID from private key: %w", err)
	}

	params, err := actors.SerializeParams(&power2.CreateMinerParams{
		Owner:         addrowner,
		Worker:        addrworker,
		SealProofType: NsRegisteredSealProof(sealProofType),
		Peer:          []byte(peerid),
	})
	if err != nil {
		return nil, err
	}
	msg := NsMessage{
		To:     builtin2.StoragePowerActorAddr,
		From:   addrowner,
		Value:  types.NewInt(0),
		Method: 2,
		Nonce:  nonce,
		Params: params,
	}

	return GenerateUtilSigMsgWithDefaultFee(prihex, msg)
}

func GenerateUtilSigMsgWithDefaultFee(prihex string, msgParam NsMessage) (*NsSignedMessage, error) {

	cp := msgParam
	msg := &cp
	var err error
	msg.GasLimit = build.BlockGasLimit
	msg.GasFeeCap = types.NewInt(uint64(build.MinimumBaseFee) + 1)
	msg.GasPremium = types.NewInt(1)
	key, err := GenerateKeyByHexString(prihex)
	if err != nil {
		return nil, err
	}
	sigture, err := SignMsg(key.NsKeyInfo, msg.Cid().Bytes())
	if err != nil {
		return nil, err
	}
	sigMsg := NsSignedMessage{
		Message:   *msg,
		Signature: *sigture,
	}
	return &sigMsg, nil
}

func DefaultFee(msg NsMessage) NsMessage {
	msg.GasLimit = build.BlockGasLimit
	msg.GasFeeCap = types.NewInt(uint64(build.MinimumBaseFee) + 1)
	msg.GasPremium = types.NewInt(1)
	return msg
}

func TestGeneratePriKey() string {
	ph, err := GeneratePriKeyHex("bls")
	if err != nil {
		fmt.Printf("GeneratePriKeyHex error %v\n", err)
		return ""
	}
	fmt.Printf("private key %s", ph)
	return ph
}

func TestGenerateCreateMinerSigMsg(ph string, sealProofType int64, nonce uint64, url string, token string) {
	addr, err := GenerateAddrByHexPri(ph)
	if err != nil {
		fmt.Printf("GenerateAddrByHexPri error %v\n", err)
		return
	}

	var close jsonrpc.ClientCloser
	var lotusApi LotusAPI
	if token != "" {
		lotusApi1, close1, err := NewLotusApi(url, token)
		if err != nil {
			fmt.Printf("NewPubLotusApi error %v\n", err)
			return
		}
		close = close1
		lotusApi = lotusApi1
	} else {
		lotusApi1, close1, err := NewPubLotusApi(url)
		if err != nil {
			fmt.Printf("NewPubLotusApi error %v\n", err)
			return
		}
		close = close1
		lotusApi = lotusApi1
	}
	defer close()
	sgm, err := GenerateCreateMinerSigMsg(lotusApi, ph, addr.String(), addr.String(), sealProofType, nonce)
	if err != nil {
		fmt.Printf("GenerateCreateMinerSigMsg error %v\n", err)
		return
	}
	fmt.Printf("private key %s\n", ph)
	fmt.Printf("address %s\n", addr)
	fmt.Printf("signtrue hex %v\n", sgm)
	cid, err := lotusApi.MpoolPush(context.TODO(), sgm)
	if err != nil {
		fmt.Printf("lotusApi MpoolPush error %v\n", err)
		return
	}
	fmt.Printf("msg cid %s\n", cid.String())
}

func GenerateWinningPoSt(ctx context.Context, actorID abi.ActorID, sectorInfo []proof2.SectorInfo, randomness abi.PoStRandomness, postProofType abi.RegisteredPoStProof, path string) ([]proof2.PoStProof, error) {
	randomness[31] &= 0x3f

	out := make([]ffi.PrivateSectorInfo, 0, len(sectorInfo))
	for _, ss := range sectorInfo {
		s := ss
		out = append(out, ffi.PrivateSectorInfo{
			CacheDirPath:     filepath.Join(path, "cache", fmt.Sprintf("s-t0%d-%d", actorID, s.SectorNumber)),
			PoStProofType:    postProofType,
			SealedSectorPath: filepath.Join(path, "sealed", fmt.Sprintf("s-t0%d-%d", actorID, s.SectorNumber)),
			SectorInfo:       s,
		})
	}

	privsectors := ffi.NewSortedPrivateSectorInfo(out...)

	return ffi.GenerateWinningPoSt(actorID, privsectors, randomness)
}

func NsaggregateSignatures(sigs []crypto.Signature) (*crypto.Signature, error) {
	sigsS := make([]ffi.Signature, len(sigs))
	for i := 0; i < len(sigs); i++ {
		copy(sigsS[i][:], sigs[i].Data[:ffi.SignatureBytes])
	}

	aggSig := ffi.Aggregate(sigsS)
	if aggSig == nil {
		if len(sigs) > 0 {
			return nil, xerrors.Errorf("bls.Aggregate returned nil with %d signatures", len(sigs))
		}

		zeroSig := ffi.CreateZeroSignature()

		// Note: for blst this condition should not happen - nil should not
		// be returned
		return &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: zeroSig[:],
		}, nil
	}
	return &crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: aggSig[:],
	}, nil
}

func NsComputeBaseFee(ctx context.Context, fa api.FullNode, ts *types.TipSet) (abi.TokenAmount, error) {
	if build.UpgradeBreezeHeight >= 0 && ts.Height() > build.UpgradeBreezeHeight && ts.Height() < build.UpgradeBreezeHeight+build.BreezeGasTampingDuration {
		return abi.NewTokenAmount(100), nil
	}

	// zero := abi.NewTokenAmount(0)

	// totalLimit is sum of GasLimits of unique messages in a tipset
	// totalLimit := int64(0)

	// seen := make(map[cid.Cid]struct{})

	// for _, b := range ts.Blocks() {
	// 	blkmsg, err := fa.ChainGetBlockMessages(ctx, b.Messages)
	// 	if err != nil {
	// 		return zero, xerrors.Errorf("getting messages for: %s: %w", b.Cid(), err)
	// 	}
	// 	msg1 := blkmsg.BlsMessages
	// 	msg2 := blkmsg.SecpkMessages
	// 	for _, m := range msg1 {
	// 		c := m.Cid()
	// 		if _, ok := seen[c]; !ok {
	// 			totalLimit += m.GasLimit
	// 			seen[c] = struct{}{}
	// 		}
	// 	}
	// 	for _, m := range msg2 {
	// 		c := m.Cid()
	// 		if _, ok := seen[c]; !ok {
	// 			totalLimit += m.Message.GasLimit
	// 			seen[c] = struct{}{}
	// 		}
	// 	}
	// }
	var totalLimit int64 = 999999999999999999
	parentBaseFee := ts.Blocks()[0].ParentBaseFee

	return store.ComputeNextBaseFee(parentBaseFee, totalLimit, len(ts.Blocks()), ts.Height()), nil
}

type cidProvider interface {
	Cid() cid.Cid
}

func BuildCid(ctx context.Context, v interface{}) (cid.Cid, error) {
	mhType := uint64(mh.BLAKE2B_MIN + 31)
	mhLen := -1
	codec := uint64(cid.DagCBOR)

	var expCid cid.Cid
	if c, ok := v.(cidProvider); ok {
		expCid := c.Cid()
		pref := expCid.Prefix()
		mhType = pref.MhType
		mhLen = pref.MhLength
		codec = pref.Codec
	}

	cm, ok := v.(cbg.CBORMarshaler)
	if ok {
		buf := new(bytes.Buffer)
		if err := cm.MarshalCBOR(buf); err != nil {
			return cid.Undef, cbor.NewSerializationError(err)
		}

		pref := cid.Prefix{
			Codec:    codec,
			MhType:   mhType,
			MhLength: mhLen,
			Version:  1,
		}
		c, err := pref.Sum(buf.Bytes())
		if err != nil {
			return cid.Undef, err
		}

		blk, err := block.NewBlockWithCid(buf.Bytes(), c)
		if err != nil {
			return cid.Undef, err
		}

		blkCid := blk.Cid()
		if expCid != cid.Undef && blkCid != expCid {
			return cid.Undef, fmt.Errorf("your object is not being serialized the way it expects to")
		}

		return blkCid, nil
	}

	nd, err := cbor.WrapObject(v, mhType, mhLen)
	if err != nil {
		return cid.Undef, err
	}

	ndCid := nd.Cid()
	if expCid != cid.Undef && ndCid != expCid {
		return cid.Undef, fmt.Errorf("your object is not being serialized the way it expects to")
	}

	return ndCid, nil
}

func NsComputeMsgMeta(bs cbor.IpldStore, bmsgCids, smsgCids []cid.Cid) (cid.Cid, error) {
	// block headers use adt0

	store := blockadt.WrapStore(context.TODO(), bs)
	bmArr := blockadt.MakeEmptyArray(store)
	smArr := blockadt.MakeEmptyArray(store)

	for i, m := range bmsgCids {
		c := cbg.CborCid(m)
		if err := bmArr.Set(uint64(i), &c); err != nil {
			return cid.Undef, err
		}
	}

	for i, m := range smsgCids {
		c := cbg.CborCid(m)
		if err := smArr.Set(uint64(i), &c); err != nil {
			return cid.Undef, err
		}
	}

	bmroot, err := bmArr.Root()
	if err != nil {
		return cid.Undef, err
	}

	smroot, err := smArr.Root()
	if err != nil {
		return cid.Undef, err
	}

	mrcid, err := store.Put(store.Context(), &types.MsgMeta{
		BlsMessages:   bmroot,
		SecpkMessages: smroot,
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to put msgmeta: %w", err)
	}

	return mrcid, nil
}
