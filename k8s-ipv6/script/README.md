To see the changes I made in calico definition to enable IPv6:
* diff calico.yaml <(curl -s https://docs.projectcalico.org/v3.7/getting-started/kubernetes/installation/hosted/calico.yaml)
* diff calicoctl.yaml <(curl -s https://docs.projectcalico.org/v3.7/getting-started/kubernetes/installation/hosted/calicoctl.yaml)
