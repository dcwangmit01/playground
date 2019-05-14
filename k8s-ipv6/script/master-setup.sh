#!/bin/bash

if [ -z "$1" -o -z "$2" -o -z "$3" -o -z "$4" ]; then
    echo Usage: $(basename $0) host-name pod-cidr svc-cidr kubeadm-token
    echo example: sh $(basename $0) kube-master-0 fd00:1234::/110 fd00:5678::/110 123456.0123456789abcdef
    exit 1
fi

HOST_NAME=$1
POD_CIDR=$2
SVC_CIDR=$3
KUBEADM_TOKEN=$4

DNS_SVC_IP=$(echo $SVC_CIDR | cut -f 1 -d /)a
ETCD_SVC_IP=$(echo $SVC_CIDR | cut -f 1 -d /)2379

set -e

kubectl get no >/dev/null 2>&1 && echo kubernetes is running && exit

HOST_IP=$(ip -6 addr show dev ens3 scope global | grep inet6 | awk '{print $2}' | cut -f 1 -d /)
hostnamectl set-hostname $HOST_NAME
sed -i "/$HOST_NAME/d" /etc/hosts
echo $HOST_IP $HOST_NAME >> /etc/hosts
sysctl -w net.ipv6.conf.all.forwarding=1

kubeadm init phase preflight
kubeadm config images pull
kubeadm init phase kubelet-start
sed -i 's/0.0.0.0/"::"/; s/10.96.0.10/"'$DNS_SVC_IP'"/' /var/lib/kubelet/config.yaml
systemctl restart kubelet
kubeadm init phase certs all \
    --apiserver-advertise-address=$HOST_IP \
    --service-cidr=$SVC_CIDR
kubeadm init phase kubeconfig all \
    --apiserver-advertise-address=$HOST_IP
kubeadm init phase control-plane all \
    --apiserver-advertise-address=$HOST_IP \
    --pod-network-cidr=$POD_CIDR \
    --service-cidr=$SVC_CIDR
kubeadm init phase etcd local

while ! curl -k https://[::]:6443/ >/dev/null 2>&1; do
    echo Waiting for cluster ...
    sleep 1
done

kubeadm init phase upload-config all
kubeadm init phase upload-certs
kubeadm init phase mark-control-plane
kubeadm init phase bootstrap-token
kubeadm init phase addon all \
    --apiserver-advertise-address=$HOST_IP \
    --pod-network-cidr=$POD_CIDR \
    --service-cidr=$SVC_CIDR

mkdir -p $HOME/.kube
cp /etc/kubernetes/admin.conf $HOME/.kube/config
chown $(id -u):$(id -g) $HOME/.kube/config

kubeadm token list \
    | grep -v TOKEN \
    | awk '{print $1}' \
    | xargs -n 1 kubeadm token delete
kubeadm token create $KUBEADM_TOKEN

curl https://docs.projectcalico.org/v3.7/getting-started/kubernetes/installation/hosted/etcd.yaml \
    | sed 's@http://$(CALICO_ETCD_IP)@http://[$(CALICO_ETCD_IP)]@;
           s@10.96.232.136@"'$ETCD_SVC_IP'"@' \
    | kubectl apply -f -
cat calico.yaml \
    | sed 's@__CALICO_IPV6POOL_CIDR__@'$POD_CIDR'@;
                    s@<ETCD_IP>:<ETCD_PORT>@['$ETCD_SVC_IP']:6666@;' \
    | kubectl apply -f -

while ! kubectl -n kube-system get sa default >/dev/null 2>&1; do
    echo Waiting for service account ...
    sleep 1
done
kubectl apply -f calicoctl.yaml

while ! kubectl -n kube-system exec calicoctl true >/dev/null 2>&1; do
    echo Waiting for calicoctl ...
    sleep 1
done

echo "---
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: kusanagi-ipv6-ippool
spec:
  blockSize: 122
  cidr: $POD_CIDR
  ipipMode: Never
  natOutgoing: true
  nodeSelector: all()
  vxlanMode: Never
" \
    | kubectl -n kube-system exec -i calicoctl -- calicoctl apply -f -

echo "---
apiVersion: projectcalico.org/v3
kind: BGPConfiguration
metadata:
  name: default
spec:
  logSeverityScreen: Info
  nodeToNodeMeshEnabled: true
  asNumber: 65432
" \
    | kubectl -n kube-system exec -i calicoctl -- calicoctl apply -f -

kubectl -n kube-system get configmap coredns -o yaml \
    | sed 's@forward.*@forward . "2001:4860:4860::8888" "2001:4860:4860::8844"@' \
    | kubectl apply -f -
kubectl -n kube-system delete po -l k8s-app=kube-dns
