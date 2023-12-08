#!/usr/bin/bash

if [ "$#" -ne 2 ]; then
    echo "Need x2 arguments"
    exit 1
fi

# https://www.man7.org/linux/man-pages/man8/tc-netem.8.html
/usr/sbin/tc qdisc replace dev "$1" root handle 1 netem delay "$2" limit 100000