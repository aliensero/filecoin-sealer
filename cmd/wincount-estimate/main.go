package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"golang.org/x/xerrors"
)

/*
	go run . 2>&1|grep "wincount [1-9]"
*/

func main() {

	miner, err := address.NewFromString("f099999") //矿工编号
	if err != nil {
		fmt.Println(xerrors.Errorf("failed to miner: %w", err))
	}

	buf := new(bytes.Buffer)
	if err := miner.MarshalCBOR(buf); err != nil {
		fmt.Println(xerrors.Errorf("failed to cbor marshal address: %w", err))
	}

	curHight := 660927 //当前高度
	for j := 10; j <= 100; j += 10 {
		for i := 1; i < 86400; i++ { //一个月86400高度
			var fr [32]byte
			for i := 0; i < 32; i++ {
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				fr[i] = byte(r.Int31())
			}
			electionRand, err := store.DrawRandomness(fr[:], crypto.DomainSeparationTag_ElectionProofProduction, abi.ChainEpoch(curHight+i), buf.Bytes())
			if err != nil {
				fmt.Println(xerrors.Errorf("failed to draw randomness: %w", err))
				continue
			}
			ep := types.ElectionProof{VRFProof: electionRand}
			p := types.NewInt(uint64(j) << 40) // TiB
			total := 10
			t := types.NewInt(uint64(total) << 60) //EiB
			z := ep.ComputeWinCount(p, t)
			fmt.Println("epoch", i+curHight, "power", types.SizeStr(p), "total", types.SizeStr(t), "wincount", z)
		}
	}
}
