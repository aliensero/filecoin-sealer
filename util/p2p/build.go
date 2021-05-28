package up2p

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"gitlab.ns/lotus-worker/util"

	"github.com/filecoin-project/go-amt-ipld/v3"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	cbg "github.com/whyrusleeping/cbor-gen"
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

func NewMpool(ps *pubsub.PubSub, nn util.NsNetworkName) *Mpool {
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

func MinerMinng(ctx context.Context, blkh *util.NsBlockHeader, fa util.LotusAPI, mr util.NsAddress, ki *util.Key, sealedPath SealedPath, parentMsg []*util.NsMessage) (*util.NsBlockHeader, error) {
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

	log.Infof("Time delta between now and our mining base: %ds", uint64(time.Now().Unix())-curTipset.MinTimestamp())

	bvals := mbi.BeaconEntries
	rbase := mbi.PrevBeaconEntry
	if len(bvals) > 0 {
		rbase = bvals[len(bvals)-1]
	}

	electionRand, err := getElectionRand(mr, rbase.Data, round)
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng failed to draw randomness: %v", err)
	}

	minerPower := mbi.MinerPower
	ticket, ep, err := getTicketAndElectionProof(ctx, mbi, electionRand, mr, round, ki, minerPower)
	if err != nil || ep.WinCount < 1 {
		return nil, xerrors.Errorf("MinerMinng getTicketAndElectionProof wincount %v error %v", ep.WinCount, err)
	}

	statOut, err := fa.StateCompute(ctx, curTipset.Height(), parentMsg, curTipset.Key())
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng StateCompute error %v", err)
	}

	blkStor := util.NsNewMemory()
	ipfsbs := util.NsNewCborStore(blkStor)
	recs := make([]cbg.CBORMarshaler, 0)
	for _, traMsg := range statOut.Trace {
		receipt := traMsg.MsgRct
		recs = append(recs, receipt)
	}
	rectroot, err := amt.FromArray(ctx, ipfsbs, recs)
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

	wpostProof, err := util.GenerateWinningPoSt(ctx, util.NsActorID(actorID), mbi.Sectors, electionRand, proofType, string(sealedPath))
	if err != nil {
		return nil, xerrors.Errorf("MinerMinng GenerateWinningPoSt miner %v error %v", mr, err)
	}

	uts := curTipset.MinTimestamp() + util.NsBlockDelaySecs
	af := time.After(time.Duration(int64(uts)-time.Now().Unix()) * time.Second)
	<-af
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

	return blkHead, nil

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

func getElectionRand(mr util.NsAddress, rebaseData []byte, round util.NsChainEpoch) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := mr.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	electionRand, err := util.NsDrawRandomness(rebaseData, util.NsDomainSeparationTag_ElectionProofProduction, round, buf.Bytes())
	if err != nil {
		return nil, err
	}
	return electionRand, nil
}

func getTicketAndElectionProof(ctx context.Context, mbi *util.NsMiningBaseInfo, electionRand []byte, mr util.NsAddress, round util.NsChainEpoch, ki *util.Key, minerPower util.Nsbig) (*util.NsTicket, *util.NsElectionProof, error) {

	vrfout, err := util.NsComputeVRF(ctx, func(ctx context.Context, addr util.NsAddress, data []byte) (*util.NsSignature, error) {
		sigture, err := util.SignMsg(ki.NsKeyInfo, electionRand)
		if err != nil {
			return nil, err
		}
		return sigture, nil
	}, util.NsAddress{}, nil)
	if err != nil {
		return nil, nil, err
	}

	ep := &util.NsElectionProof{VRFProof: vrfout}
	j := ep.ComputeWinCount(minerPower, mbi.NetworkPower)
	ep.WinCount = j
	log.Infof("wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww round %v curPower %v NetworkPower %v sectors %v count %d", round, minerPower, mbi.NetworkPower, mbi.Sectors, ep.WinCount)

	ticket := &util.NsTicket{
		VRFProof: vrfout,
	}
	return ticket, ep, nil
}

func publishBlockMsg(ctx context.Context, fa util.LotusAPI, blkHead *util.NsBlockHeader, msgs []*util.NsSignedMessage, ki *util.Key, pub *pubsub.PubSub, topic string) error {
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

	// baseFee, err := util.NsComputeBaseFee(ctx, fa, curTipset)
	// if err != nil {
	// 	return err
	// }
	// blkHead.ParentBaseFee = baseFee
	blkHead.ParentBaseFee = util.NsNewInt(100)

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
	err = pub.Publish(topic, b)
	if err != nil {
		return err
	}
	log.Infof("publishBlockMsg cid %v ParentStateRoot %v ParentMessageReceipts %v ParentBaseFee %v len(msgs) %v", blk.Cid(), blkHead.ParentStateRoot, blkHead.ParentMessageReceipts, blkHead.ParentBaseFee, len(msgs))
	for _, msg := range msgs {
		log.Infof("msg cid %v", msg.Cid())
	}
	return nil
}

func GenerateWinningFallbackSectorChallenges(mr util.NsAddress, rebaseData []byte, round util.NsChainEpoch, mbi *util.NsMiningBaseInfo) (*util.NsFallbackChallenges, error) {
	rand, err := getElectionRand(mr, rebaseData, round)
	if err != nil {
		return nil, err
	}
	proofType, err := util.NsWinningPoStProofTypeFromWindowPoStProofType(255, util.NsRegisteredPoStProof(mbi.Sectors[0].SealProof))
	if err != nil {
		return nil, err
	}
	actorID, err := util.NsIDFromAddress(mr)
	sectorIds := make([]util.NsSectorNum, len(mbi.Sectors))
	for i, s := range mbi.Sectors {
		sectorIds[i] = s.SectorNumber
	}
	return util.NsGeneratePoStFallbackSectorChallenges(proofType, util.NsActorID(actorID), rand, sectorIds)
}

func GenerateSingleWinningVanillaProof(
	path string,
	mr util.NsAddress,
	poStProofType util.NsRegisteredPoStProof,
	rebaseData []byte, round util.NsChainEpoch,
	mbi *util.NsMiningBaseInfo,
) (map[util.NsSectorNum][]byte, error) {
	actorID, err := util.NsIDFromAddress(mr)
	if err != nil {
		return nil, err
	}
	challages, err := GenerateWinningFallbackSectorChallenges(mr, rebaseData, round, mbi)
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
