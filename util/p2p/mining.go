package up2p

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"gitlab.ns/lotus-worker/util"

	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/lib/peermgr"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/xerrors"
)

type Mpool struct {
	Msgs []*util.NsSignedMessage
}

func (mp *Mpool) AppendMsg(m *util.NsSignedMessage) {
	mp.Msgs = append(mp.Msgs, m)
}

func (mp *Mpool) ClearMsg() {
	mp.Msgs = []*util.NsSignedMessage{}
}

func NewMpool(ps *pubsub.PubSub, nn util.NsNetworkName, pem *peermgr.PeerMgr) *Mpool {
	mp := &Mpool{}
	go func() {
		msgTopic := util.NsMessageTopic(nn)
		ps.RegisterTopicValidator(msgTopic, func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			m, err := util.NsDecodeSignedMessage(msg.Message.GetData())
			if err != nil {
				log.Warnf("failed to decode incoming message: %s", err)
				return pubsub.ValidationReject
			}
			msg.ValidatorData = m
			pem.AddFilecoinPeer(pid)
			return pubsub.ValidationAccept
		})
		sub, err := ps.Subscribe(msgTopic)
		if err != nil {
			log.Errorf("Subscribe topic %v error %v", msgTopic, err)
			panic(err)
		}
		for {
			msg, err := sub.Next(context.TODO())
			if err != nil {
				log.Errorf("sub.Next error %v", err)
				continue
			}
			if msg.ValidatorData != nil {
				sm, ok := msg.ValidatorData.(*util.NsSignedMessage)
				log.Infof("block NsSignedMessage ok? %v", ok)
				if ok {
					mp.AppendMsg(sm)
				}
			}
		}
	}()
	return mp
}

type PrivateKey string
type SealedPath string

type MiningResult struct {
	BlockHeader *util.NsBlockHeader
	BaseTipSet  *util.NsTipSet
	Ticket      *util.NsTicket
}

func MinerMinng(ctx context.Context, blkh *util.NsBlockHeader, fa util.LotusAPI, mr util.NsAddress, ki *util.Key, sealedPath SealedPath, parentMsg []*util.NsMessage, mapMsg map[util.NsCid]*util.NsMessage) (*MiningResult, error) {
	curTipset, err := fa.ChainHead(ctx)
	for {
		if err != nil {
			return nil, xerrors.Errorf("MinerMinng ChainHead error %v", err)
		}
		if curTipset.Height() == blkh.Height {
			break
		}
		if curTipset.Height() > blkh.Height {
			return nil, xerrors.Errorf("MinerMinng ChainHead curTipset.Height(%v) > blkh.Height(%v)", curTipset.Height(), blkh.Height)
		}
		curTipset, err = fa.ChainHead(ctx)
	}
	mbi, round, err := getBaseInfo(ctx, blkh, fa, mr, curTipset)
	if err != nil || mbi == nil {
		return nil, xerrors.Errorf("MinerMinng get miner %v info mbi == nil %v error %v", mr, mbi == nil, err)
	}

	log.Infof("Time delta between now and our mining base: %ds", uint64(build.Clock.Now().Unix())-curTipset.MinTimestamp())

	bvals := mbi.BeaconEntries
	rbase := mbi.PrevBeaconEntry
	if len(bvals) > 0 {
		rbase = bvals[len(bvals)-1]
	}

	ticket, err := getTicketProof(ctx, rbase.Data, round, mr, ki, curTipset)
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng failed to draw randomness: %v", err)
	}

	minerPower := mbi.MinerPower
	ep, err := getElectionProof(ctx, mbi, rbase.Data, round, mr, ki, minerPower)
	if err != nil || ep.WinCount < 1 {
		return nil, xerrors.Errorf("MinerMinng getTicketAndElectionProof wincount %v error %v", ep.WinCount, err)
	}
	statOut, err := fa.StateCompute(ctx, curTipset.Height(), parentMsg, curTipset.Key())
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng StateCompute error %v", err)
	}

	ipfsbs := util.NsNewMemCborStore()
	arrStor := util.NsMakeEmptyArray(util.NsWrapStore(ctx, ipfsbs))
	i := 0
	for _, traMsg := range statOut.Trace {
		if _, ok := mapMsg[traMsg.MsgCid]; ok {
			log.Warnf("receipt heigth %v cid %v receipt %#v", curTipset.Height(), traMsg.MsgCid, traMsg.MsgRct)
			err := arrStor.Set(uint64(i), traMsg.MsgRct)
			if err != nil {
				return nil, err
			}
			i++
		}
	}
	rectroot, err := arrStor.Root()
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng failed to build receipts amt: %v", err)
	}

	actorID, err := util.NsIDFromAddress(mr)
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng NsIDFromAddress miner %v error %v", mr, err)
	}

	proofType, err := util.NsWinningPoStProofTypeFromWindowPoStProofType(255, util.NsRegisteredPoStProof(mbi.Sectors[0].SealProof))
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng determining winning post proof type: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := mr.MarshalCBOR(buf); err != nil {
		return nil, xerrors.Errorf("failed to marshal miner address: %w", err)
	}

	rand, err := util.NsDrawRandomness(rbase.Data, util.NsDomainSeparationTag_WinningPoStChallengeSeed, round, buf.Bytes())
	if err != nil {
		return nil, xerrors.Errorf("failed to get randomness for winning post: %w", err)
	}

	prand := util.NsPoStRandomness(rand)
	wpostProof, err := util.GenerateWinningPoSt(ctx, util.NsActorID(actorID), mbi.Sectors, prand, proofType, string(sealedPath))
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng GenerateWinningPoSt miner %v error %v", mr, err)
	}

	uts := curTipset.MinTimestamp() + util.NsBlockDelaySecs
	blkHead := &util.NsBlockHeader{
		Miner:         mr,
		Parents:       curTipset.Key().Cids(),
		Ticket:        ticket,
		ElectionProof: ep,

		BeaconEntries:         mbi.BeaconEntries,
		Height:                round,
		Timestamp:             uts,
		WinPoStProof:          wpostProof,
		ParentStateRoot:       statOut.Root,
		ParentMessageReceipts: rectroot,
	}

	return &MiningResult{blkHead, curTipset, ticket}, nil

}

func getBaseInfo(ctx context.Context, blkh *util.NsBlockHeader, fa util.LotusAPI, mr util.NsAddress, curTipset *util.NsTipSet) (*util.NsMiningBaseInfo, util.NsChainEpoch, error) {
	tps, err := util.NsNewTipSet([]*util.NsBlockHeader{blkh})
	if err != nil {
		return nil, 0, err
	}
	round := tps.Height() + util.NsChainEpoch(1)
	mbi, err := fa.MinerGetBaseInfo(ctx, mr, round, curTipset.Key())
	return mbi, round, err
}

func getTicketProof(ctx context.Context, rebaseData []byte, round util.NsChainEpoch, mr util.NsAddress, ki *util.Key, pts *util.NsTipSet) (*util.NsTicket, error) {
	buf := new(bytes.Buffer)
	if err := mr.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	if round > build.UpgradeSmokeHeight {
		buf.Write(pts.MinTicket().VRFProof)
	}

	ticketRand, err := util.NsDrawRandomness(rebaseData, util.NsDomainSeparationTag_TicketProduction, round-build.TicketRandomnessLookback, buf.Bytes())
	if err != nil {
		return nil, err
	}

	vrfOut, err := util.NsComputeVRF(ctx, func(ctx context.Context, addr util.NsAddress, data []byte) (*util.NsSignature, error) {
		sigture, err := util.SignMsg(ki.NsKeyInfo, ticketRand)
		if err != nil {
			return nil, err
		}
		return sigture, nil
	}, ki.Address, ticketRand)
	if err != nil {
		return nil, err
	}

	ticket := &util.NsTicket{
		VRFProof: vrfOut,
	}
	return ticket, nil
}

func getElectionProof(ctx context.Context, mbi *util.NsMiningBaseInfo, rebaseData []byte, round util.NsChainEpoch, mr util.NsAddress, ki *util.Key, minerPower util.Nsbig) (*util.NsElectionProof, error) {

	buf := new(bytes.Buffer)
	if err := mr.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	electionRand, err := util.NsDrawRandomness(rebaseData, util.NsDomainSeparationTag_ElectionProofProduction, round, buf.Bytes())
	if err != nil {
		return nil, err
	}

	vrfout, err := util.NsComputeVRF(ctx, func(ctx context.Context, addr util.NsAddress, data []byte) (*util.NsSignature, error) {
		sigture, err := util.SignMsg(ki.NsKeyInfo, electionRand)
		if err != nil {
			return nil, err
		}
		return sigture, nil
	}, ki.Address, electionRand)
	if err != nil {
		return nil, err
	}

	ep := &util.NsElectionProof{VRFProof: vrfout}
	j := ep.ComputeWinCount(minerPower, mbi.NetworkPower)
	ep.WinCount = j
	log.Infof("wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww round %v curPower %v NetworkPower %v sectors %v count %d", round, minerPower, mbi.NetworkPower, mbi.Sectors, ep.WinCount)

	return ep, nil
}

func publishBlockMsg(ctx context.Context, fa util.LotusAPI, miningRet *MiningResult, cbs util.NsBlockstore, pmsg []*util.NsMessage, ki *util.Key, pub *pubsub.PubSub, topic string) error {

	blkHead := miningRet.BlockHeader
	curts := miningRet.BaseTipSet
	msgs, err := fa.MpoolSelect(context.TODO(), miningRet.BaseTipSet.Key(), miningRet.Ticket.Quality())
	if err != nil {
		return xerrors.Errorf("failed to select messages for block: %w", err)
	}

	var blk util.NsBlockMsg
	var blsSigs []util.NsSignature

	blockstore := util.NsNewMemory()
	bs := util.NsNewCborStore(blockstore)
	var blkcids []util.NsCid
	var secpkcids []util.NsCid
	for _, msg := range msgs {
		if msg.Signature.Type == util.NsSigTypeBLS {
			blsSigs = append(blsSigs, msg.Signature)
			blk.BlsMessages = append(blk.BlsMessages, msg.Cid())
			b, err := msg.Message.ToStorageBlock()
			if err != nil {
				return xerrors.Errorf("publishBlockMsg ToStorageBlock error %v", err)
			}
			blkcids = append(blkcids, b.Cid())
		} else {
			blk.SecpkMessages = append(blk.SecpkMessages, msg.Cid())
			b, err := msg.ToStorageBlock()
			if err != nil {
				return xerrors.Errorf("publishBlockMsg Secpk ToStorageBlock error %v", err)
			}
			secpkcids = append(secpkcids, b.Cid())
		}
	}

	blsmsgroot, err := util.NsComputeMsgMeta(bs, blkcids, secpkcids)
	if err != nil {
		return xerrors.Errorf("publishBlockMsg NsComputeMsgMeta error %v", err)
	}
	blkHead.Messages = blsmsgroot

	tsKey := util.NsNewTipSetKey(blkHead.Parents...)
	parentWeight, err := fa.ChainTipSetWeight(ctx, tsKey)
	if err != nil {
		return err
	}
	blkHead.ParentWeight = parentWeight

	totalLimit := int64(0)
	for _, m := range pmsg {
		totalLimit += m.GasLimit
	}
	blkHead.ParentBaseFee = util.NsComputeNextBaseFee(curts.Blocks()[0].ParentBaseFee, totalLimit, len(curts.Blocks()), curts.Height())

	aggSig, err := util.NsaggregateSignatures(blsSigs)
	if err != nil {
		return err
	}
	blkHead.BLSAggregate = aggSig

	hbuf, err := blkHead.Serialize()
	if err != nil {
		return err
	}
	sigture, err := util.SignMsg(ki.NsKeyInfo, hbuf)
	if err != nil {
		return err
	}
	blkHead.BlockSig = sigture

	blk.Header = blkHead

	b, err := blk.Serialize()
	if err != nil {
		return err
	}

	bmsgs := make([]*util.NsMessage, 0)
	smsgs := make([]*util.NsSignedMessage, 0)
	for _, msg := range msgs {
		cm := *msg
		if msg.Signature.Type == util.NsSigTypeBLS {
			bmsgs = append(bmsgs, &cm.Message)
		} else {
			smsgs = append(smsgs, &cm)
		}
	}
	err = SaveBlock(ctx, cbs, bmsgs, smsgs, blk.Header)
	if err != nil {
		log.Errorf("SaveBlock error %v", err)
		return err
	}

	deadline := blk.Header.Timestamp - 1
	baseT := time.Unix(int64(deadline), 0)
	build.Clock.Sleep(build.Clock.Until(baseT))

	err = pub.Publish(topic, b)
	if err != nil {
		return err
	}

	log.Infof("publishBlockMsg cid %v ParentStateRoot %v ParentMessageReceipts %v ParentBaseFee %v len(msgs) %v waitPublish %v", blk.Cid(), blkHead.ParentStateRoot, blkHead.ParentMessageReceipts, blkHead.ParentBaseFee, len(msgs), baseT)
	for _, msg := range msgs {
		log.Infof("msg cid %v", msg.Cid())
	}
	return nil
}

func GenerateWinningFallbackSectorChallenges(mr util.NsAddress, rebaseData []byte, round util.NsChainEpoch, mbi *util.NsMiningBaseInfo, pts *util.NsTipSet) (*util.NsFallbackChallenges, error) {

	proofType, err := util.NsWinningPoStProofTypeFromWindowPoStProofType(255, util.NsRegisteredPoStProof(mbi.Sectors[0].SealProof))
	if err != nil {
		return nil, err
	}
	actorID, err := util.NsIDFromAddress(mr)
	sectorIds := make([]util.NsSectorNum, len(mbi.Sectors))
	for i, s := range mbi.Sectors {
		sectorIds[i] = s.SectorNumber
	}
	return util.NsGeneratePoStFallbackSectorChallenges(proofType, util.NsActorID(actorID), nil, sectorIds)
}

func GenerateSingleWinningVanillaProof(
	path string,
	mr util.NsAddress,
	poStProofType util.NsRegisteredPoStProof,
	rebaseData []byte, round util.NsChainEpoch,
	mbi *util.NsMiningBaseInfo,
	pts *util.NsTipSet,
) (map[util.NsSectorNum][]byte, error) {
	actorID, err := util.NsIDFromAddress(mr)
	if err != nil {
		return nil, err
	}
	challages, err := GenerateWinningFallbackSectorChallenges(mr, rebaseData, round, mbi, pts)
	if err != nil {
		return nil, err
	}
	retMap := make(map[util.NsSectorNum][]byte)
	for _, si := range mbi.Sectors {
		_, err := os.Stat(fmt.Sprintf("%s/cache/s-t0%d-%d/p_aux", path, actorID, si.SectorNumber))
		if err != nil {
			continue
		}
		privateSectorInfo := util.NsPrivateSectorInfo{}
		privateSectorInfo.SectorInfo = util.NsSectorInfo{
			SealProof:    si.SealProof,
			SectorNumber: si.SectorNumber,
			SealedCID:    si.SealedCID,
		}
		privateSectorInfo.CacheDirPath = fmt.Sprintf("%s/cache/s-t0%d-%d", path, actorID, si.SectorNumber)
		privateSectorInfo.PoStProofType = poStProofType
		privateSectorInfo.SealedSectorPath = fmt.Sprintf("%s/sealed/s-t0%d-%d", path, actorID, si.SectorNumber)
		vp, err := util.NsGenerateSingleVanillaProof(privateSectorInfo, challages.Challenges[si.SectorNumber])
		if err != nil {
			return nil, err
		}
		retMap[si.SectorNumber] = vp
	}
	return retMap, nil
}

func GenerateWinningPoStWithVanilla(
	proofType util.NsRegisteredPoStProof,
	minerID util.NsActorID,
	randomness []byte,
	proofs [][]byte,
) {

}
