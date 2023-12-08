#!/usr/bin/bash

# https://www.man7.org/linux/man-pages/man8/tc-netem.8.html
/usr/sbin/tc qdisc replace dev "$1" root handle 1 netem loss 100%