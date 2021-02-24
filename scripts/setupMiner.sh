#!/bin/bash

actorID=1033
actor="t0$actorID"
minerRepoBase="/root/miner_storage"
minerRepo="$minerRepoBase/$actor/.lotus-miner"
prihex="7b2254797065223a22626c73222c22507269766174654b6579223a222b5a5364547652794a4949786769697259334c636f30654343674b4251716e2f61533677594a396f4243593d227d"
lotusMinerPath=/root/lotus

export LOTUS_MINER_PATH=$minerRepo
which lotus-miner
if [ $? -ne 0 ];then
        export PATH=$PATH:$lotusMinerPath
fi

lotus-miner --miner-repo $minerRepo init --actor $actor --nosync --no-local-storage=true

nohup lotus-miner --miner-repo $minerRepo run --miner-api 1$actorID --nosync --prihex $prihex --store "$minerRepoBase/$actor" > log.miner-$actor 2>&1 &