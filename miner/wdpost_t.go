package miner

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (m *Miner) TestStartSubmitPoSt(trnmsg util.PoStTransfer) (int, error) {
	proofs := trnmsg.Proofs
	PoStProofType := trnmsg.PoStProofType
	randomness := trnmsg.Randomness
	minerID := int64(trnmsg.MinerID)
	allProveBitfield := trnmsg.AllProveBitfield

	vanillaMap := make(map[uint64][]byte)
	for _, vp := range proofs {
		vanillaMap[vp.SectorNum] = vp.Proof
	}
	sortedProofs := make([][]byte, 0, len(proofs))
	allProveBitfield.ForEach(func(sid uint64) error {
		sortedProofs = append(sortedProofs, vanillaMap[sid])
		return nil
	})
	starkProofs, err := util.NsGenerateWindowPoStWithVanilla(
		PoStProofType,
		util.NsActorID(minerID),
		randomness,
		sortedProofs,
	)
	return len(starkProofs), err
}

func (m *Miner) TestWindowPostProofs(minerID uint64,
	sectorInfoPath string, path string, proofTyep int64, challange int64, deadlineIndex int64, partInx int, prihex string) (int, error) {

	f, err := os.Open(sectorInfoPath)
	if err != nil {
		return 0, err
	}
	fbuf, err := ioutil.ReadAll(f)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(fbuf), "\n")
	sec_commr := make(map[uint64]string)
	for _, l := range lines {
		ls := strings.Split(l, " ")
		if len(ls) < 2 {
			continue
		}
		s, err := strconv.ParseUint(ls[0], 10, 64)
		if err != nil {
			continue
		}
		sec_commr[s] = ls[1]
	}

	log.Infof("sectors info len %v, %v", len(sec_commr), sec_commr)

	miner, err := util.NsNewIDAddress(uint64(minerID))
	if err != nil {
		return 0, err
	}

	buf := new(bytes.Buffer)
	if err := miner.MarshalCBOR(buf); err != nil {
		log.Error(xerrors.Errorf("failed to marshal address to cbor: %w", err))
		return 0, err
	}

	ts, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		log.Error(xerrors.Errorf("WinPoStServer failed ChainGetTipSetByHeight: %w", err))
		return 0, err
	}
	key := ts.Key()
	echal := util.NsChainEpoch(challange)
	rand, err := m.LotusApi.ChainGetRandomnessFromBeacon(context.TODO(), key, util.NsDomainSeparationTag_WindowedPoStChallengeSeed, echal, buf.Bytes())
	if err != nil {
		return 0, err
	}
	randomness := util.NsPoStRandomness(rand)
	randomness[31] &= 0x3f
	log.Infof("PoStRandomness %v", randomness)

	spi, ok := util.NsSealProofInfos[util.NsRegisteredSealProof(proofTyep)]
	if !ok {
		return 0, xerrors.Errorf("SealProofInfos[%v] don't exist", proofTyep)
	}
	var pis []util.NsPrivateSectorInfo
	var sis []util.NsSectorInfo
	for si, comm := range sec_commr {
		cid, err := util.NsCidDecode(comm)
		if err != nil {
			return 0, err
		}
		s := util.NsSectorInfo{
			SealProof:    util.NsRegisteredSealProof(proofTyep),
			SectorNumber: util.NsSectorNum(si),
			SealedCID:    cid,
		}
		pii := util.NsPrivateSectorInfo{
			CacheDirPath:     fmt.Sprintf("%s/cache/s-t0%d-%d", path, minerID, si),
			PoStProofType:    spi.WindowPoStProof,
			SealedSectorPath: fmt.Sprintf("%s/sealed/s-t0%d-%d", path, minerID, si),
			SectorInfo:       s,
		}
		pis = append(pis, pii)
		sis = append(sis, s)
	}

	privsectors := util.NsNewSortedPrivateSectorInfo(pis...)
	proofs, _, err := util.NsGenerateWindowPoSt(util.NsActorID(minerID), privsectors, randomness)
	if err != nil {
		return 0, err
	}
	log.Info(proofs)
	vi := util.NsWindowPoStVerifyInfo{
		Randomness:        randomness,
		Proofs:            proofs,
		Prover:            util.NsActorID(minerID),
		ChallengedSectors: sis,
	}
	ok, err = util.NsVerifyWindowPoSt(vi)
	log.Infof("VerifyWindowPost ok %v? error %v", ok, err)

	if prihex != "" {
		actorID, err := util.NsNewIDAddress(minerID)
		if err != nil {
			return 0, err
		}
		ret, err := m.submitPostMessage(actorID, prihex, echal, deadlineIndex, partInx, proofs)
		if err != nil {
			return 0, err
		}
		log.Infof("submitPostMessage result %v", ret)
	}

	return len(sec_commr), nil
}

func (m *Miner) TestVanillaProof(minerID int64, sectorNums []util.NsSectorNum, commr []string, storagePath string, pt util.NsRegisteredSealProof) error {

	postType := util.NsSealProofInfos[pt].WindowPoStProof
	randomness := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, randomness)
	if err != nil {
		return err
	}
	randomness[31] &= 0x3f

	fcs, err := util.NsGeneratePoStFallbackSectorChallenges(
		postType,
		util.NsActorID(minerID),
		randomness,
		sectorNums,
	)
	if err != nil {
		return err
	}

	// var vanillaProofs [][]byte
	// var sis []util.NsSectorInfo
	// var proofs []util.NsPoStProof
	for i, s := range sectorNums {
		snum := s
		cid, err := util.NsCidDecode(commr[i])
		if err != nil {
			log.Error(err)
			continue
		}
		si := util.NsSectorInfo{
			SealProof:    pt,
			SectorNumber: snum,
			SealedCID:    cid,
		}
		pii := util.NsPrivateSectorInfo{
			CacheDirPath:     fmt.Sprintf("%s/cache/s-t0%d-%d", storagePath, minerID, snum),
			PoStProofType:    postType,
			SealedSectorPath: fmt.Sprintf("%s/sealed/s-t0%d-%d", storagePath, minerID, snum),
			SectorInfo:       si,
		}
		p, err := util.NsGenerateSingleVanillaProof(
			pii,
			fcs.Challenges[snum],
		)
		if err != nil {
			log.Error(err)
		}
		vanillaProofs := [][]byte{p}
		_, err = util.NsGenerateWindowPoStWithVanilla(postType, util.NsActorID(minerID), randomness, vanillaProofs)
		if err != nil {
			return err
		}
		// log.Infof("snark proof len %v", len(proof[0].ProofBytes))
		// proofs = append(proofs, proof...)
		// sis = append(sis, si)
	}
	// vi := util.NsWindowPoStVerifyInfo{
	// 	Randomness:        randomness,
	// 	Proofs:            proofs,
	// 	Prover:            util.NsActorID(minerID),
	// 	ChallengedSectors: sis,
	// }
	// ok, err := util.NsVerifyWindowPoSt(vi)
	// log.Infof("VerifyWindowPost ok %v? error %v", ok, err)

	return err
}
