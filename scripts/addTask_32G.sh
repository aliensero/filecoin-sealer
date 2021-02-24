#!/bin/bash
taskCnt=10;
actorID=1018;
unsealedPath="/var/tmp/filecoin-proof-parameters/unsealed-34359738368";
#unsealedPath="/var/tmp/filecoin-proof-parameters/unsealed-536870912";
storagePath="/root/miner_storage";
sealedProof="32GiB";
#sealedProof="";
proofType=8;
#proofType=-1;
pieceStr="baga6ea4seaqao7s73y24kcutaosvacpdjgfe5pw76ooefnyqw4ynr3d2y6x2mpq";
#pieceStr="";
addr="127.0.0.1:4321";
sectorbeg=0;
for ((i=0;i<$taskCnt;i++));do
	sectornum=$(( sectorbeg + i ));
	jsonData='{ "jsonrpc": "2.0", "method": "NSMINER.AddTask", "params": ['$actorID','$sectornum',"PC1","'$storagePath'","'$unsealedPath'","'$sealedProof'",'$proofType',"'$pieceStr'"], "id": 1 }';
    curl -X POST \
	 -H "Content-Type: application/json" \
	 --data "$jsonData" \
	 "http://$addr/rpc/v0";
done
