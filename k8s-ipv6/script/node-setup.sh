#!/bin/bash

if [ -z "$1" -o -z "$2" -o -z "$3" -o -z "$4" ]; then
    echo Usage:
    echo '    '$(basename $0) host-name master-ip kubeadm-token
    echo Example:
    echo '    'sh $(basename $0) kube-node-0 2600:1f18:6685:7903:173b:bd68:761f:4bd9 fd00:5678::a 123456.0123456789abcdef
    exit 1
fi

HOST_NAME=$1
MASTER_IP=$2
DNS_SVC_IP=$3
KUBEADM_TOKEN=$4

set -e

HOST_IP=$(ip -6 addr show dev ens3 scope global | grep inet6 | awk '{print $2}' | cut -f 1 -d /)
hostnamectl set-hostname $HOST_NAME
sed -i "/$HOST_NAME/d" /etc/hosts
echo $HOST_IP $HOST_NAME >> /etc/hosts
sysctl -w net.ipv6.conf.all.forwarding=1

kubeadm join [$MASTER_IP]:6443 --token $KUBEADM_TOKEN --discovery-token-unsafe-skip-ca-verification
sed -i 's/0.0.0.0/"::"/; s/10.96.0.10/"'$DNS_SVC_IP'"/' /var/lib/kubelet/config.yaml
systemctl restart kubelet
