#!/bin/bash


if [ "$1" == "-h" ];then
        echo "[nodeType,default=0] [minerapi,default=ws://127.0.0.1:4321/rpc/v0] [image,default=ns-worker:latest]";
        exit;
fi

nodeType=${1-0};
minerapi=${2-"ws://127.0.0.1:4321/rpc/v0"};
image=${3-"ns-worker:latest"};
begPort=8755;
cpusetstart=60;

if [ $nodeType -eq 0 ];then
        echo "nodeType $nodeType for test";
	pc1cnt=3;
	pc2cnt=2;
	c1cnt=1;
	c2cnt=1;
	pc2gpubeg=0;
	pc2gpucnt=1;
	c2gpubeg=1;
	c2gpucnt=1;
	binvolume="-v /root/ns-worker/cmd/worker/worker:/usr/local/bin/worker";
elif [ $nodeType -eq 1 ];then
        echo "nodeType $nodeType for test";
	pc1cnt=3;
	pc2cnt=2;
	c1cnt=1;
	c2cnt=0;
	pc2gpubeg=0;
	pc2gpucnt=1;
	c2gpubeg=1;
	c2gpucnt=1;
elif [ $nodeType -eq 2 ];then
        echo "nodeType $nodeType for test";
	pc1cnt=3;
	pc2cnt=2;
	c1cnt=1;
	c2cnt=0;
	pc2gpubeg=0;
	pc2gpucnt=-1;
	c2gpubeg=1;
	c2gpucnt=1;
elif [ $nodeType -eq 3 ];then
	pc1cnt=3;
	pc2cnt=2;
	c1cnt=1;
	c2cnt=1;
	pc2gpubeg=0;
	pc2gpucnt=-1;
	c2gpubeg=1;
	c2gpucnt=-1;
elif [ $nodeType -eq 4 ];then
	postcnt=1;
	binvolume="-v /root/ns-worker/cmd/worker/worker:/usr/local/bin/worker";
fi

workerid=`hostname`
eth0=`netstat -r|grep default|awk '{print $8}'`;
extrip=`ifconfig|grep -C1 $eth0|tail -n 1|awk '{print $2}'`;
cpuset=(`lscpu -e|grep -v CPU|awk '{print $1":"$2":"$5}'|awk -F: '{print $1" "$2" "$6}'|sort -k 1 -k 3 -n`);

runfunc(){

        name=${1-"pc1-1"};
        port=${2};
        cpuset=${3};
        nvidia=${4};
        cpumem=${5};
        lotusapi=${6};
        lotustoken=${7};
        cpusetenv="--cpuset-cpus=$cpuset";
        cpumemenv="--cpuset-mems=$cpumem";
        nvidiaenv="--runtime nvidia -e NVIDIA_VISIBLE_DEVICES=$nvidia -e FIL_PROOFS_USE_GPU_COLUMN_BUILDER=1 -e FIL_PROOFS_USE_GPU_TREE_BUILDER=1";
        extrlisten="";

        if [ ! -n "$cpuset" ];then
                cpusetenv="";
        fi

        if [ ! -n "$cpumem" ];then
                cpumemenv=""
        fi

        if [[ ! "$name" =~ "c2" ]] || [ ! -n "$nvidia" ];then
                nvidiaenv="";
        fi

        if [[ "$name" =~ "post" ]] && [ -n "$nvidia" ];then
		nvidiaenv="--runtime nvidia -e NVIDIA_VISIBLE_DEVICES=$nvidia -e FIL_PROOFS_USE_GPU_COLUMN_BUILDER=1 -e FIL_PROOFS_USE_GPU_TREE_BUILDER=1";
        fi

        if [ -n "$port" ];then
                extrlisten="--extrlisten $extrip:$port"
        fi

	if [ -n "$lotusapi" ] && [ -n "$lotustoken" ];then
		lotusparams="--lotusapi $lotusapi --lotustoken $lotustoken";
	fi

        echo "docker run -d --hostname $name --name docker-$name $nvidiaenv $cpusetenv $cpumemenv -e RUST_BACKTRACE=1 -e RUST_LOG=trace -e FIL_PROOFS_MAXIMIZE_CACHING=1 -e FIL_PROOFS_USE_MULTICORE_SDR=1 -e FIL_PROOFS_MULTICORE_SDR_PRODUCERS=2 -v /var/tmp:/var/tmp -v /root/miner_storage:/root/miner_storage $binvolume -p $port:3456 $image run --minerapi $minerapi --listen 0.0.0.0:3456 --workerid $workerid $extrlisten $lotusparams"
        docker run -d --hostname $name --name docker-$name $nvidiaenv $cpusetenv $cpumemenv -e RUST_BACKTRACE=1 -e RUST_LOG=trace -e FIL_PROOFS_MAXIMIZE_CACHING=1 -e FIL_PROOFS_USE_MULTICORE_SDR=1 -e FIL_PROOFS_MULTICORE_SDR_PRODUCERS=2 -v /var/tmp:/var/tmp -v /root/miner_storage:/root/miner_storage $binvolume -p $port:3456 $image run --minerapi $minerapi --listen 0.0.0.0:3456 --workerid $workerid $extrlisten $lotusparams
}

i=0;
cpusetlen=${#cpuset[@]};
cpuset=(${cpuset[*]:cpusetstart:cpusetlen});
cpusetlen=${#cpuset[@]};
half=$(( cpusetlen/2 ));
pc1cnt=${pc1cnt-0};
while [ $i -lt $pc1cnt ];do
        s=$((i/2));
        y=$((i%2));
        if [ $y -eq 0 ];then
                b=${cpuset[$(($s*12))]};
                n=${cpuset[$(($s*12+1))]};
                e=${cpuset[$(($s*12+6))]};
        fi
        if [ $y -ne 0 ];then
                b=${cpuset[$(($s*12+$half))]};
                n=${cpuset[$(($s*12+1+$half))]};
                e=${cpuset[$(($s*12+6+$half))]};
        fi
        cpubeg=$b;
        cpuend=$e;
        echo "runfunc pc1-$begPort-$i $begPort $cpubeg-$cpuend '' $n";
        runfunc "pc1-$begPort-$i" $begPort $cpubeg-$cpuend "" $n;
        begPort=$(( begPort + 1 ));
        i=$(( i + 1 ));
done

i=0;
pc2gpucnt=${pc2gpucnt--1};
gpusize=$(( pc2cnt/pc2gpucnt|bc ));
pc2cnt=${pc2cnt-0};
while [ $i -lt $pc2cnt ];do
	if [ $pc2gpucnt -ne -1 ];then
		gpu=$(( pc2gpubeg + i/gpusize|bc  ));
		echo "runfunc pc2-$begPort-$i $begPort '' $gpu";
		runfunc "pc2-$begPort-$i" $begPort "" $gpu;
	else
		echo "runfunc pc2-$begPort-$i $begPort";
		runfunc "pc2-$begPort-$i" $begPort;
	fi
        begPort=$(( begPort + 1 ));
        i=$(( i+1 ));
done

i=0;
c1cnt=${c1cnt-0};
while [ $i -lt $c1cnt ];do
	echo "runfunc c1-$begPort-$i $begPort";
        runfunc "c1-$begPort-$i" $begPort;
        begPort=$(( begPort + 1 ));
        i=$(( i+1 ));
done

i=0;
c2gpucnt=${c2gpucnt--1};
gpusize=$(( c2cnt/c2gpucnt|bc ));
c2cnt=${c2cnt-0};
while [ $i -lt $c2cnt ];do
	if [ $c2gpucnt -ne -1 ];then
		gpu=$(( c2gpubeg + i/gpusize|bc  ));
		echo "runfunc c2-$begPort-$i $begPort '' $gpu";
		runfunc "c2-$begPort-$i" $begPort "" $gpu;
	else
		echo "runfunc c2-$begPort-$i $begPort";
		runfunc "c2-$begPort-$i" $begPort;
	fi
        begPort=$(( begPort + 1 ));
        i=$(( i+1 ));
done
i=0;
postcnt=${postcnt-0};
while [ $i -lt $postcnt ];do
	echo "runfunc post-$begPort-$i $begPort '' 0";
        runfunc "post-$begPort-$i" $begPort "" 0 "" "ws://127.0.0.1:1234/rpc/v0" "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.uIW9_CIAGVrNmeB97ousRXfGrMpqR4B-u5HFNPz4fgY";
        begPort=$(( begPort + 1 ));
        i=$(( i+1 ));
done
