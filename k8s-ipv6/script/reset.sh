#!/bin/bash

set -e
kubeadm reset -f
rm -rfv /var/lib/calico /var/etcd
iptables -F
iptables -t nat -F
iptables -t mangle -F
iptables -X
