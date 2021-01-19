#!/bin/sh

# $1 = filename of output file
# $2 = address where output is sent

cat $1
cat $1 | curl -X POST --data-binary @$1 $2

echo "DONE"
