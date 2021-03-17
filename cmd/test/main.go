package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/docker/go-units"
	"gitlab.ns/lotus-worker/util"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Println("[cache_dir] [sealed_dir]")
		return
	}
	fmt.Printf("cache_dir %s sealed_dir %s\n", args[1], args[2])
	RecoverSealedFile("baga6ea4seaqdsvqopmj2soyhujb72jza76t4wpq5fzifvm3ctz47iyytkewnubq", "512MiB", 7, args[1], "/var/tmp/filecoin-proof-parameters/unsealed-536870912", args[2], 200, 1044, "dcca61251469eb6a5d74b0d6ae59d3666752f2ffae78faf5cb14ef9134bd1820")
}

func RecoverSealedFile(piececid string, sealerProof string, proofType int64, cacheDirPath, stagedSectorPath, sealedSectorPath string, sectorNum int64, actorID int64, ticketHex string) {

	sectorSizeInt, err := units.RAMInBytes(sealerProof)

	if err != nil {
		fmt.Println(xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err))
		return
	}

	pieces, err := util.NewNsPieceInfo(piececid, sectorSizeInt)
	if err != nil {
		fmt.Println(err)
		return
	}
	ticketBtyes, err := hex.DecodeString(ticketHex)
	if err != nil {
		fmt.Println(err)
		return
	}
	ticket := util.NsSealRandomness(ticketBtyes[:])
	fproofType := util.NsRegisteredSealProof(proofType)
	fsectorNum := util.NsSectorNum(sectorNum)
	minerID := util.NsActorID(actorID)
	phase1Output, err := util.NsSealPreCommitPhase1(fproofType, cacheDirPath, stagedSectorPath, sealedSectorPath, fsectorNum, minerID, ticket, pieces)
	if err != nil {
		fmt.Println(err)
		return
	}
	sealedCID, unsealedCID, err := util.NsSealPreCommitPhase2(phase1Output, cacheDirPath, sealedSectorPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("comm_d %v comm_r %v", unsealedCID, sealedCID)
}

type NoSchedulingObj struct {
	Noschs []NoSchedulingInfo
}

type NoSchedulingInfo struct {
	Hostname  string
	Tasktypes []string
}

type ti interface {
	ti1()
}

type tis1 struct{}

func (t tis1) ti1() {
	fmt.Println("tis1")
}

type tis2 struct {
	ti
}

func invokt(s s1) {
	fmt.Println(s)
}

type param struct {
	fx.In
	Lifecycle fx.Lifecycle
	Name      string
}

type s1 struct {
	name string
}

func News1(p param) s1 {
	ret := s1{p.Name}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			fmt.Printf("ret %v\n", p.Name)
			return nil
		},
	})
	return ret
}

func testDB() {

	db, err := util.InitMysql("dockeruser", "password", "127.0.0.1", "3306", "db_worker")
	if err != nil {
		fmt.Println(err)
		return
	}

	var taskInfo util.DbTaskInfo
	err = db.Debug().Where("actor_id=1034 and sector_num=333").Find(&taskInfo).Error
	if err != nil {
		fmt.Errorf("%v\n", err)
		return
	}
	fmt.Printf("%s\n", string(taskInfo.C1Out))
	phase2Out, err := util.NsSealCommitPhase2(taskInfo.C1Out, util.NsSectorNum(*taskInfo.SectorNum), util.NsActorID(*taskInfo.ActorID))
	if err != nil {
		fmt.Errorf("%v\n", err)
		return
	}
	fmt.Println(phase2Out)
}

func test() ([]byte, error) {
	err := errors.New("test error")
	return []byte{1, 2}, err
}

func TestSigture() {
	// ph, err := util.GeneratePriKeyHex("bls")
	// fmt.Printf("private key %s error1 %v\n", ph, err)
	// ph := "7b2254797065223a22626c73222c22507269766174654b6579223a224665577258514a48436762354c4d346d685733734f736e447278622b5a6d4670467a74496269754870426b3d227d"
	// addr, err2 := util.GenerateAddrByHexPri(ph)
	// fmt.Printf("address %s err2 %v\n", addr, err2)
	// util.TestGenerateCreateMinerSigMsg("7b2254797065223a22626c73222c22507269766174654b6579223a224665577258514a48436762354c4d346d685733734f736e447278622b5a6d4670467a74496269754870426b3d227d", 8, 3, "http://127.0.0.1:1234/rpc/v0", "")
	util.TestGenerateCreateMinerSigMsg("7b2254797065223a22626c73222c22507269766174654b6579223a224665577258514a48436762354c4d346d685733734f736e447278622b5a6d4670467a74496269754870426b3d227d", 8, 2, "https://calibration.node.glif.io/rpc/v0", "")

}
