#!/bin/bash

if [ 2 -gt $# ]; then
        echo "params [actorID] [worker api]";
        exit;
fi
actorID=${1:-1004};
addr=${2:-"127.0.0.1:3456"};

jsonData1='{ "jsonrpc": "2.0", "method": "NSWORKER.RetryTaskPIDByState", "params": ['$actorID',"C2","/usr/local/bin/worker"], "id": 1 }'
jsonData='{ "jsonrpc": "2.0", "method": "NSWORKER.ProcessCommitPhase2", "params": ['$actorID',-1,"/usr/local/bin/worker"], "id": 1 }'
while true ;do
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data  "$jsonData1" \
	 "http://$addr/rpc/v0"
    sleep 1;
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data  "$jsonData" \
	 "http://$addr/rpc/v0"
    sleep 2;
done;