package worker

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/lotus/api"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

const (
	PUB = "pub"
	PRI = "pri"
)

type ActorPoStInfo struct {
	AddrInfo string
	AddrType string
	DiOpen   util.NsChainEpoch
}

func (w *Worker) AddPoStActor(actorID int64, addrinfo string, addrtype string) (string, error) {
	id, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return "", err
	}
	w.PoStMiner[id] = ActorPoStInfo{AddrInfo: addrinfo, AddrType: addrtype}
	log.Infof("AddPoStActor %v", id.String())
	return id.String(), nil
}

func (w *Worker) DisplayPoStActor() (string, error) {
	key := "PoSt actors:"
	for k, _ := range w.PoStMiner {
		key = key + " " + k.String()
	}
	return key, nil
}

func (w *Worker) WinPoStServer() {
	key := util.NsTipSetKey{}
	if w.LotusApi == nil {
		log.Warnf("WinPoStServer Setup WinPostServer failed lotusApi is nil")
		return
	}

	var notifs <-chan []*api.HeadChange
	for {
		var err error
		if notifs == nil {
			notifs, err = w.LotusApi.ChainNotify(context.TODO())
			if err != nil {
				log.Errorf("WinPoStServer ChainNotify error: %+v", err)
				if strings.Contains(err.Error(), "websocket routine exiting") {
					lapi, _, err := util.NewLotusApi(w.LotusUrl, w.LoutsToken)
					if err != nil {
						log.Errorf("WinPoStServer NewLotusApi error %v", err)
					}
					w.LotusApi = lapi
					err = w.ConnMiner()
					if err != nil {
						log.Errorf("WinPoStServer ConnMiner error %v", err)
					}
				}
				time.Sleep(10 * time.Second)
				continue
			}
		}
		changes, ok := <-notifs
		if !ok {
			log.Warn("WinPoStServer window post notifs channel closed")
			notifs = nil
			continue
		}
		if len(changes) == 0 {
			log.Errorf("WinPoStServer window post first notif to have len > 0")
			continue
		}
		tmpKey := util.NsTipSetKey{}
		for _, change := range changes {
			if change.Val == nil {
				log.Errorf("WinPoStServer change.Val was nil")
				continue
			}
			if change.Type == "current" {
				tmpKey = change.Val.Key()
				break
			}
		}

		if tmpKey.String() == key.String() && !key.IsEmpty() {
			continue
		}
		key = tmpKey
		for a, info := range w.PoStMiner {
			ta := a
			tinfo := info

			di, err := w.LotusApi.StateMinerProvingDeadline(context.TODO(), a, key)
			if di.Open != info.DiOpen && di.CurrentEpoch >= di.Open && di.CurrentEpoch < di.Close {
				log.Infof("WinPoStServer actorID %v deadline %v StateMinerProvingDeadline", a, di)
				go w.SearchPartitions(di, a, info.AddrInfo, info.AddrType)
				tinfo.DiOpen = di.Open
				w.PoStMiner[ta] = tinfo
			} else {
				log.Warnf("WinPoStServer already exec actorID %v deadline %v error %v", a, di, err)
			}
		}
	}
}

func (w *Worker) SearchPartitions(di *util.NsDeadLineInfo, actorID util.NsAddress, addrInfo string, addrType string) error {
	ts, err := w.LotusApi.ChainGetTipSetByHeight(context.TODO(), di.Challenge, util.NsTipSetKey{})
	if err != nil {
		log.Error(xerrors.Errorf("WinPoStServer failed ChainGetTipSetByHeight: %w", err))
		return err
	}
	key := ts.Key()
	id, err := util.NsIDFromAddress(actorID)
	if err != nil {
		log.Error(xerrors.Errorf("WinPoStServer failed to tranger address to id: %w", err))
		return err
	}

	partitions, err := w.LotusApi.StateMinerPartitions(context.TODO(), actorID, di.Index, key)
	if err != nil {
		log.Errorf("WinPoStServer SearchPartitions actorID %v error %v", actorID, err)
		return err
	}
	buf := new(bytes.Buffer)
	if err := actorID.MarshalCBOR(buf); err != nil {
		log.Error(xerrors.Errorf("failed to marshal address to cbor: %w", err))
		return err
	}
	log.Infof("WinPoStServer SearchPartitions util.NsTipSetKey %v len(partitions)=%d", key, len(partitions))
	if len(partitions) == 0 {
		return xerrors.Errorf("WinPoStServer len(partitions) == 0")
	}
	rand, err := w.LotusApi.ChainGetRandomnessFromBeacon(context.TODO(), key, util.NsDomainSeparationTag_WindowedPoStChallengeSeed, di.Challenge, buf.Bytes())
	if err != nil {
		log.Errorf("WinPoStServer SearchPartitions actorID %v ChainGetRandomnessFromBeacon error %v", actorID, err)
		return err
	}
	for partInx, part := range partitions {
		go w.GeneratePoStProof(*di, id, part, util.NsPoStRandomness(rand), addrInfo, addrType, partInx)
	}
	return nil
}

func (w *Worker) GeneratePoStProof(di util.NsDeadLineInfo, minerID uint64, partition util.NsPartition, randomness util.NsPoStRandomness, addrInfo string, addrType string, partInx int) {

	// toProve, err := bitfield.SubtractBitField(partition.LiveSectors, partition.FaultySectors)
	// if err != nil {
	// 	log.Errorf("GeneratePoStProof bitfield.SubtractBitField ")
	// }
	var err error
	var addr util.NsAddress
	var toProve bitfield.BitField
	var repoProve bitfield.BitField
	var fault bitfield.BitField
	var proofs []util.VanillaProof
	var PoStProofType util.NsRegisteredPoStProof
	fcs := (*util.NsFallbackChallenges)(nil)

	addr, err = util.NsNewIDAddress(minerID)
	if err != nil {
		log.Warnf("PostErr NsNewIDAddress workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		err = xerrors.Errorf("PostErr NsNewIDAddress workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		return
	}

	for i := 0; i < 5; i++ {
		if w.MinerApi != nil && w.MinerApi.CheckServer() {
			break
		}
		err := w.ConnMiner()
		if err != nil {
			log.Errorf("PoStErr w.queryPoSt workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		}
		time.Sleep(3 * time.Second)
	}

	defer func() {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		trnmsg := util.PoStTransfer{
			WorkerID:         w.WorkerID,
			HostName:         w.HostName,
			Actor:            addr,
			MinerID:          minerID,
			AddrInfo:         addrInfo,
			AddrType:         addrType,
			Dealine:          di.Index,
			CurrentEpoch:     int64(di.CurrentEpoch),
			CloseEpoch:       int64(di.Close),
			Challenge:        int64(di.Challenge),
			AllProveBitfield: toProve,
			ToProveBitfield:  repoProve,
			FaultBitfield:    fault,
			Proofs:           proofs,
			PartitionIndex:   partInx,
			Randomness:       randomness,
			PoStProofType:    PoStProofType,
			Err:              errMsg,
		}
		if w.MinerApi != nil {
			minerHandle := w.MinerApi.StartSubmitPoSt(trnmsg)
			log.Infof("PoStInfo result minerID %d deadline %d partitionIndex %d error %v submit to miner %s", minerID, di.Index, partInx, err, minerHandle)
		} else {
			log.Infof("PoStInfo result minerID %d deadline %d partitionIndex %d error %v", minerID, di.Index, partInx, err)
		}
	}()

	toProve, err = bitfield.SubtractBitField(partition.LiveSectors, partition.FaultySectors)
	if err != nil {
		err = xerrors.Errorf("PoStErr bitfield.SubtractBitField(partition.LiveSectors,partition.FaultySectors) workerid %s hostname %s minerID %v deadline %d error %v", w.WorkerID, w.HostName, minerID, di.Index, err)
		return
	}
	toProve, err = bitfield.MergeBitFields(toProve, partition.RecoveringSectors)
	if err != nil {
		err = xerrors.Errorf("PoStErr bitfield.MergeBitFields(partition.LiveSectors,partition.FaultySectors) workerid %s hostname %s minerID %v deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		return
	}
	whr := ""
	sset := make([]util.NsSectorNum, 0, 1024)
	toProve.ForEach(func(sid uint64) error {
		if whr == "" {
			whr = fmt.Sprintf("%d", sid)
		} else {
			whr = fmt.Sprintf("%s,%d", whr, sid)
		}
		sset = append(sset, util.NsSectorNum(sid))
		return nil
	})
	reqInfo := util.RequestInfo{
		ActorID:  int64(minerID),
		WorkerID: w.WorkerID,
		Sectors:  whr,
	}
	postInfos, err := w.queryPoSt(reqInfo)
	if err != nil || len(postInfos) == 0 {
		log.Errorf("PoStInfo QueryToPoSt minerID %d deadline %d partitionIdex %d len(postInfos) %d error %v", minerID, di.Index, partInx, len(postInfos), err)
		err = xerrors.Errorf("PoStInfo QueryToPoSt minerID %d deadline %d partitionIdex %d len(postInfos) %d error %v", minerID, di.Index, partInx, len(postInfos), err)
		return
	}
	log.Infof("PoStInfo QueryToPoSt minerID %d deadline %d partitionIdex %d len(postInfos) %d", minerID, di.Index, partInx, len(postInfos))
	tMap := make(map[uint64]util.DbPostInfo)
	repoProve = bitfield.New()
	for _, postInfo := range postInfos {
		pi := postInfo
		repoProve.Set(uint64(pi.SectorNum))
		sectorNum := uint64(pi.SectorNum)
		tMap[sectorNum] = pi
	}
	slen, err := repoProve.Count()
	if err != nil {
		log.Errorf("PostErr repoProve.Count workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		err = xerrors.Errorf("PostErr repoProve.Count workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		return
	}

	fault = bitfield.New()
	proofs = make([]util.VanillaProof, 0, slen)

	randomness[31] &= 0x3f

	trepoProve := repoProve
	trepoProve.ForEach(func(sid uint64) error {
		cid, err1 := util.NsCidDecode(tMap[sid].CommR)
		if err1 != nil {
			err = xerrors.Errorf("PostErr repoProve.ForEach workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err1)
			return err1
		}

		if fcs == nil {
			PoStProofType = util.NsRegisteredPoStProof(tMap[sid].ProofType)
			fcs, err1 = util.NsGeneratePoStFallbackSectorChallenges(
				util.NsRegisteredPoStProof(PoStProofType),
				util.NsActorID(minerID),
				randomness,
				sset,
			)
			if err1 != nil {
				err = xerrors.Errorf("PostErr NsGeneratePoStFallbackSectorChallenges workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, err1)
				return err1
			}
			log.Infof("PoStInfo minerID %d deadline %d partitionIndex %d challenge %v", di.Index, partInx, fcs)
		}

		s := util.NsSectorInfo{
			SealProof:    util.NsRegisteredSealProof(tMap[sid].ProofType),
			SectorNumber: util.NsSectorNum(tMap[sid].SectorNum),
			SealedCID:    cid,
		}
		pii := util.NsPrivateSectorInfo{
			CacheDirPath:     tMap[sid].CacheDirPath,
			PoStProofType:    PoStProofType,
			SealedSectorPath: tMap[sid].SealedSectorPath,
			SectorInfo:       s,
		}
		p, err := util.NsGenerateSingleVanillaProof(
			pii,
			fcs.Challenges[util.NsSectorNum(sid)],
		)
		if err != nil {
			repoProve.Unset(uint64(sid))
			fault.Set(uint64(sid))
			log.Warnf("PoStErr NsGenerateSingleVanillaProof minerID %d deadline %d partitionIndex %d sectorNum %d error %v", minerID, di.Index, partInx, sid, err)
			return nil
		}
		vp := util.VanillaProof{
			SectorNum: sid,
			Proof:     p,
		}
		proofs = append(proofs, vp)
		return nil
	})

}

func (w *Worker) RetryWinPoSt(actorID int64, addrInfo string, addrTyep string) (string, error) {
	a, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return "", err
	}
	if w.LotusApi == nil {
		return "", xerrors.Errorf("w.LotusApi is nil")
	}
	tipset, err := w.LotusApi.ChainHead(context.TODO())
	if err != nil {
		if strings.Contains(err.Error(), "websocket routine exiting") {
			lapi, _, err := util.NewLotusApi(w.LotusUrl, w.LoutsToken)
			if err != nil {
				log.Errorf("NewLotusApi error %v", err)
			}
			w.LotusApi = lapi
		}
		return "", err
	}
	key := tipset.Key()
	di, err := w.LotusApi.StateMinerProvingDeadline(context.TODO(), a, key)
	if di.CurrentEpoch >= di.Open && di.CurrentEpoch < di.Close {
		err := w.SearchPartitions(di, a, addrInfo, addrTyep)
		if err != nil {
			return "", err
		}
	} else {
		return "", xerrors.Errorf("actorID %d di.CurrentEpoch(%d) < di.Open(%d) || di.CurrentEpoch(%d) > di.Close(%d)", actorID, di.CurrentEpoch, di.Open, di.CurrentEpoch, di.Close)
	}
	return fmt.Sprintf("actorID %d di.Index %d di.CurrentEpoch %d di.Open %d di.Close %d ", actorID, di.Index, di.CurrentEpoch, di.Open, di.Close), nil
}
