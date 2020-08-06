#!/bin/sh

pid=`ps x -opid= | sed -n 2p`
pid=`echo $pid | xargs`
ps aux
tail -f /proc/$pid/root/$1 > /tmp/out.1 &
tailp=$!

while test -d /proc/$pid; do sleep 5; done

kill $tailp

ls -l /tmp/
/parser/* /tmp/out.1

exit 0
