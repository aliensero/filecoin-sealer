#!/bin/bash

if [ 2 -gt $# ]; then
        echo "params [actorID] [miner api]";
        exit;
fi
actorID=${1:-1004};
addr=${2:-"127.0.0.1:4321"};

jsonData='{ "jsonrpc": "2.0", "method": "NSMINER.GetSeedRand", "params": ['$actorID',-1], "id": 1 }'

while true;do
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data "$jsonData" \
	 "http://$addr/rpc/v0"
    sleep 3;
done;