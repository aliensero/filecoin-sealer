#!/bin/bash
if [ $# -ne 2 ];then
	echo "[actorID] [clearOpt]";
	exit;
fi

actorID=${1:-1036};
clearOpt=${2:-"noclear"};
cat server-$actorID.pid |awk '{print "kill -9 "$3}'|sh -C

if [ $clearOpt == "clear" ];then
    rm -f *$actorID.log
    rm -f server-$actorID.pid
fi
