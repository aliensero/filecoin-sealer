#!/bin/bash

if [ "$1" == "-h" ];then
        echo "[optType] [nodeType,default=0] [actorID] [minerAPI] [privatekeyhex] [sendFIL] [feeFIL]";
        exit;
fi

optType=${1};
nodeType=${2-0};
actorID=${3-1034};
minerAPI=${4-"127.0.0.1:4321"};
prihex=${5-"7b2254797065223a22626c73222c22507269766174654b6579223a22546d32433249506337624a7243584e583331585a366b54794e54414f625234396a336a5846576d52356b773d227d"};
value=${6-"0"};
fee=${7-"0"};
if [ $nodeType -eq 0 ];then
        echo "nodeType $nodeType for test";
	pc1cnt=3
	pc2cnt=2
	c1cnt=1
	c2cnt=1
	precnt=1
	seedcnt=1
	commitcnt=1
elif [ $nodeType -eq 1 ];then
        echo "nodeType $nodeType for test";
	pc1cnt=3
	pc2cnt=2
	c1cnt=1
	c2cnt=0
	precnt=0
	seedcnt=0
	commitcnt=0
fi

begPort=8740;
pidfile="server-$actorID.pid";
datef=`date '+%Y%m%d%H%M%S'`;

runfunc(){
        workertype=${1-""};
        port=${2-3457}
        taskType=${3-""}
	workerip=${4-"127.0.0.1"};
        if [ "" == "$workertype" -o "$workertype" == "pc1" ];then
                echo "bash pc1Run.sh $actorID $workerip:$port";
                nohup bash pc1Run.sh $actorID "$workerip:$port" >> pc1Run-$datef-$actorID.log 2>&1 & echo "pc1 $datef-$actorID $!" >> $pidfile
                echo "setup pc1Run.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "pc2" ];then
                echo "bash pc2Run.sh $actorID $workerip:$port";
                nohup bash pc2Run.sh $actorID "$workerip:$port" >> pc2Run-$datef-$actorID.log 2>&1 & echo "pc2 $datef-$actorID $!" >> $pidfile
                echo "setup pc2Run.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "pre" ];then
                if [ "$prihex" == "" ];then
                        echo "bash preCommitSend.sh $actorID $minerAPI $value $fee";
                        nohup bash preCommitSend.sh $actorID $minerAPI $value $fee >> precommitSend-$datef-$actorID.log 2>&1 & echo "precommitSend $datef-$actorID $!" >> $pidfile
                else
                        echo "bash preCommitSendBypri.sh $actorID $minerAPI $value $prihex";
                        nohup bash preCommitSendBypri.sh $actorID $minerAPI $value $prihex >> precommitSend-$datef-$actorID.log 2>&1 & echo "precommitSend $datef-$actorID $!" >> $pidfile
                fi
                echo "setup precommitSend.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "seed" ];then
                echo "bash seedGet.sh $actorID $minerAPI";
                nohup bash seedGet.sh $actorID $minerAPI >> seedGet-$datef-$actorID.log 2>&1 & echo "seedGet $datef-$actorID $!" >> $pidfile
                echo "setup seedGet.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "c1" ];then
                echo "bash c1Run.sh $actorID $workerip:$port";
                nohup bash c1Run.sh $actorID "$workerip:$port" >> c1Run-$datef-$actorID.log 2>&1 & echo "c1 $datef-$actorID $!" >> $pidfile
                echo "setup c1Run.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "c2" ];then
                echo "bash c2Run.sh $actorID $workerip:$port";
                nohup bash c2Run.sh $actorID "$workerip:$port" >> c2Run-$datef-$actorID.log 2>&1 & echo "c2 $datef-$actorID $!" >> $pidfile
                echo "setup c2Run.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "retry" ];then
                echo "bash retryRun.sh $actorID $workerip:$port $taskType";
                nohup bash retryRun.sh $actorID "$workerip:$port" $taskType >> retryRun-$datef-$actorID.log 2>&1 & echo "retry $datef-$actorID $!" >> $pidfile
                echo "setup retryRun.sh complite "$?;
        fi

        if [ "" == "$workertype" -o "$workertype" == "commit" ];then
                if [ "$prihex" == "" ];then
                        echo "bash commitSend.sh $actorID $minerAPI $value $fee";
                        nohup bash commitSend.sh $actorID $minerAPI $value $fee >> commitSend-$datef-$actorID.log 2>&1 & echo "commitSend $datef-$actorID $!" >> $pidfile
                else
                        echo "bash commitSendBypri.sh $actorID $minerAPI $value $prihex";
                        nohup bash commitSendBypri.sh $actorID $minerAPI $value $prihex >> commitSend-$datef-$actorID.log 2>&1 & echo "commitSend $datef-$actorID $!" >> $pidfile
                fi
                echo "setup commitSend.sh complite "$?;
        fi
}

pc1cnt=${pc1cnt-0};
pc2cnt=${pc1cnt-0};
c1cnt=${c1cnt-0};
c2cnt=${c2cnt-0};
precnt=${precnt-0};
seedcnt=${seedcnt-0};
commitcnt=${commitcnt-0};

if [ "$optType" == "docker" ];then
        i=0;
        while [ $i -lt $pc1cnt ];do
                #runfunc "retry" $begPort "PC1";
                runfunc "pc1" $begPort;
                begPort=$(( begPort + 1 ));
                i=$(( i+1 ));
        done
        i=0;
        while [ $i -lt $pc2cnt ];do
                #runfunc "retry" $begPort "PC2";
                runfunc "pc2" $begPort;
                begPort=$(( begPort + 1 ));
                i=$(( i+1 ));
        done
        i=0;
        while [ $i -lt $c1cnt ];do
                #runfunc "retry" $begPort "C1";
                runfunc "c1" $begPort;
                begPort=$(( begPort + 1 ));
                i=$(( i+1 ));
        done
        i=0;
        while [ $i -lt $c2cnt ];do
                #runfunc "retry" $begPort "C2";
                runfunc "c2" $begPort;
                begPort=$(( begPort + 1 ));
                i=$(( i+1 ));
        done
        i=0;
        while [ $i -lt $precnt ];do
                runfunc pre;
                i=$(( i+1 ));
        done
        i=0;
        while [ $i -lt $seedcnt ];do
                runfunc seed;
                i=$(( i+1 ));
        done
        i=0;
        while [ $i -lt $commitcnt ];do
                runfunc commit;
                i=$(( i+1 ));
        done
else
        runfunc;
fi
