#!/bin/bash

if [ 4 -gt $# ]; then
        echo "params [actorID] [miner api] [send FIL] [fee]";
        exit;
fi
actorID=${1:-1004};
addr=${2:-"127.0.0.1:4321"};
value=${3:-"0"};
fee=${4:-"0"};

jsonData='{ "jsonrpc": "2.0", "method": "NSMINER.SendCommit", "params": ['$actorID',-1,"'$value'","'$fee'"], "id": 1 }'

while true;do
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data "$jsonData" \
	 "http://$addr/rpc/v0"
    sleep 3;
done;