package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/term"
	"golang.org/x/xerrors"
)

var log = logging.Logger("trial-mining")

func main() {

	if level, ok := os.LookupEnv("LEVEL_LOG"); !ok {
		logging.SetLogLevel("*", "INFO")
	} else {
		logging.SetLogLevel("*", level)
	}

	local := []*cli.Command{
		runCmd,
		testCmd,
	}
	app := &cli.App{
		Name:     "trial-mining",
		Commands: local,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%+v", err)
		return
	}
}

var runCmd = &cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "prihex",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := util.NsGetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()
		miner := cctx.Args().First()

		ctx := cctx.Context
		var aft time.Duration = 0
		var curH util.NsChainEpoch = 0
		for {
			<-time.After(aft)
			curTipset, err := api.ChainHead(ctx)
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}
			if curH >= curTipset.Height() {
				continue
			}
			trialWinning(ctx, miner, curTipset.Height(), api, cctx.String("prihex"))
			toSleep := uint64(time.Now().Unix()) - curTipset.MinTimestamp()
			aft = time.Duration(30-toSleep) * time.Second
			curH = curTipset.Height()
		}
	},
}

var testCmd = &cli.Command{
	Name:  "test",
	Usage: "<begin epoch> <end epoch> <miner>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "prihex",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := util.NewPubLotusApi(os.Getenv("FULLNODE_API_INFO"))
		if err != nil {
			return err
		}
		defer closer()
		ctx := cctx.Context
		hs := cctx.Args().First()
		hs1 := cctx.Args().Get(1)
		bhi, err := strconv.ParseInt(hs, 10, 64)
		if err != nil {
			return err
		}
		ehi, err := strconv.ParseInt(hs1, 10, 64)
		if err != nil {
			return err
		}
		pk := cctx.String("prihex")

		if pk == "" {
			fmt.Print("enter private key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			fmt.Println()
			pk = string(buf)
		}
		for i := bhi; i <= ehi; i++ {
			curTipset, err := api.ChainGetTipSetByHeight(ctx, util.NsChainEpoch(i), util.NsTipSetKey{})
			if err != nil {
				log.Error(err)
				continue
			}
			err = trialWinning(ctx, cctx.Args().Get(2), curTipset.Height(), api, pk)
			if err != nil {
				log.Error(err)
			}
		}
		return nil
	},
}

func trialWinning(ctx context.Context, miner string, height util.NsChainEpoch, api util.LotusAPI, prihex string) error {
	addr, err := util.NsNewFromString(miner)
	if err != nil {
		return err
	}
	round := util.NsChainEpoch(height)
	tps, err := api.ChainGetTipSetByHeight(ctx, round-1, util.NsTipSetKey{})
	if err != nil {
		return err
	}
	mbi, err := api.MinerGetBaseInfo(ctx, addr, round, tps.Key())
	if err != nil {
		return xerrors.Errorf("failed to get mining base info: %w", err)
	}
	if mbi == nil {
		return xerrors.Errorf("failed to get mining base info: %w", err)
	}
	beaconPrev := mbi.PrevBeaconEntry
	if len(mbi.BeaconEntries) > 0 {
		beaconPrev = mbi.BeaconEntries[len(mbi.BeaconEntries)-1]
	}

	ret, err := IsRoundWinner(ctx, tps, round, addr, beaconPrev, mbi, prihex)
	if err != nil {
		return err
	}
	buf, err := json.Marshal(ret)
	if err != nil {
		return err
	}
	log.Infof("%v", string(buf))
	return nil
}

type RoundMiningInfo struct {
	Miner        string            `json:"miner"`
	WinningCount int64             `json:"winning_count"`
	VRFValue     util.Nsbig        `json:"vrf_value"`
	Height       util.NsChainEpoch `json:"height"`
	MinerPower   util.Nsbig        `json:"miner_power"`
	NetworkPower util.Nsbig        `json:"network_power"`
}

func IsRoundWinner(ctx context.Context, ts *util.NsTipSet, round util.NsChainEpoch,
	miner util.NsAddress, brand util.NsBeaconEntry, mbi *util.NsMiningBaseInfo, prihex string) (RoundMiningInfo, error) {

	buf := new(bytes.Buffer)
	if err := miner.MarshalCBOR(buf); err != nil {
		return RoundMiningInfo{}, xerrors.Errorf("failed to cbor marshal address: %w", err)
	}

	electionRand, err := util.NsDrawRandomness(brand.Data, util.NsDomainSeparationTag_ElectionProofProduction, round, buf.Bytes())
	if err != nil {
		return RoundMiningInfo{}, xerrors.Errorf("failed to draw randomness: %w", err)
	}

	vrfout, err := util.NsComputeVRF(ctx, func(ctx context.Context, m util.NsAddress, data []byte) (*util.NsSignature, error) {
		ki, err := util.GenerateKeyByHexString(prihex)
		if err != nil {
			return nil, err
		}
		sigture, err := util.SignMsg(ki.NsKeyInfo, electionRand)
		if err != nil {
			return nil, err
		}
		return sigture, nil
	}, mbi.WorkerKey, electionRand)
	if err != nil {
		return RoundMiningInfo{}, xerrors.Errorf("failed to compute VRF: %w", err)
	}

	ep := &util.NsElectionProof{VRFProof: vrfout}
	j := ep.ComputeWinCount(mbi.MinerPower, mbi.NetworkPower)
	h := blake2b.Sum256(vrfout)
	lhs := util.NsBigFromBytes(h[:])
	return RoundMiningInfo{miner.String(), j, lhs, round, mbi.MinerPower, mbi.NetworkPower}, nil
}
