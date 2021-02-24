#!/bin/bash

if [ 4 -gt $# ]; then
        echo "params [actorID] [miner api] [send FIL] [prihex]";
        exit;
fi
actorID=${1:-1004};
addr=${2:-"127.0.0.1:4321"};
value=${3:-"0"};
prihex=${4:-""};

jsonData='{ "jsonrpc": "2.0", "method": "NSMINER.SendCommitByPrivatKey", "params": ["'$prihex'",'$actorID',-1,"'$value'"], "id": 1 }'

while true;do
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data "$jsonData" \
	 "http://$addr/rpc/v0"
    sleep 3;
done;