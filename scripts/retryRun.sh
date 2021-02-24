#/bin/bash

if [ 3 -gt $# ]; then
        echo "params [actorID] [worker api] [taskType]";
        exit;
fi
actorID=${1:-1004};
addr=${2:-"127.0.0.1:3456"};
taskType=${3:-"PC1"};

jsonData='{ "jsonrpc": "2.0", "method": "NSWORKER.RetryTaskPIDByState", "params": ['$actorID',"'$taskType'","/usr/local/bin/worker"], "id": 1 }'
while true ;do
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data  "$jsonData" \
	 "http://$addr/rpc/v0"
    sleep 3;
done;