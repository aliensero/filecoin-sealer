package worker

import (
	"encoding/hex"
	"fmt"

	"github.com/docker/go-units"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func RecoverSealedFile(piececid string, sealerProof string, proofType int64, cacheDirPath, stagedSectorPath, sealedSectorPath string, sectorNum int64, actorID int64, ticketHex string, seedHex string) error {

	sectorSizeInt, err := units.RAMInBytes(sealerProof)

	if err != nil {
		fmt.Println(xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err))
		return err
	}

	pieces, err := util.NewNsPieceInfo(piececid, sectorSizeInt)
	if err != nil {
		fmt.Println(err)
		return err
	}
	ticketBtyes, err := hex.DecodeString(ticketHex)
	if err != nil {
		fmt.Println(err)
		return err
	}
	ticket := util.NsSealRandomness(ticketBtyes[:])
	fproofType := util.NsRegisteredSealProof(proofType)
	fsectorNum := util.NsSectorNum(sectorNum)
	minerID := util.NsActorID(actorID)
	phase1Output, err := util.NsSealPreCommitPhase1(fproofType, cacheDirPath, stagedSectorPath, sealedSectorPath, fsectorNum, minerID, ticket, pieces)
	if err != nil {
		fmt.Println(err)
		return err
	}
	sealedCID, unsealedCID, err := util.NsSealPreCommitPhase2(phase1Output, cacheDirPath, sealedSectorPath)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Printf("comm_d %v comm_r %v\n", unsealedCID, sealedCID)
	seedBytes, err := hex.DecodeString(seedHex)
	if err != nil {
		fmt.Println(err)
		return err
	}
	_, err = util.NsSealCommitPhase1(fproofType, sealedCID, unsealedCID, cacheDirPath, sealedSectorPath, util.NsSectorNum(sectorNum), util.NsActorID(actorID), ticket, util.NsSeed(seedBytes[:]), pieces)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
