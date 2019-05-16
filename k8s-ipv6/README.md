## How to Use

### AWS
* Go to terraform/ then run: 
  ```
  terraform apply -var aws-access-key=access-key -var aws-secret-key=secret-key -var aws-keypair=hanxie -var env-name=hanxie-test -auto-approve
  ```
  This will bring up 1 jump host, 1 kube master, 2 kube nodes
* Copy script/ to kube-master and kube-node
  * You need to use jump-host as bastion to reach kube master and kube nodes
* On kube master, go to the script/ directory, run
  ```
  sudo -i
  sh master-setup.sh kube-master-0 fd00:1234::/110 fd00:5678::/110 123456.0123456789abcdef
  ```
  * `kube-master-0`: name of the master
  * `fd00:1234::/110`: pod CIDR
  * `fd00:5678::/110`: service CIDR
  * `123456.0123456789abcdef`: kubeadm token, you can use this to get a better one:
     ```
     echo $(openssl rand -hex 6).$(openssl rand -hex 16)
     ```
  If you want to know changes I made for IPv6:
  ```
   diff script/calico.yaml <(curl -s https://docs.projectcalico.org/v3.7/getting-started/kubernetes/installation/hosted/calico.yaml)
   diff script/calicoctl.yaml <(curl -s https://docs.projectcalico.org/v3.7/getting-started/kubernetes/installation/hosted/calicoctl.yaml)
   ````
* On each kube node, go to the script/ directory, run
  ```
  sudo -i
  sh node-setup.sh kube-node-name IPv6-of-master fd00:5678::a 123456.0123456789abcdef
  ```
  * `kube-node-name`: name of the kube node, needs to be unique within the same cluster
  * `IPv6-of-master`: IPv6 Address of kube master
  * `fd00:5678::a`: kube DNS' service IP, it has to be the 10th IP in service CIDR used by kube master
  * `123456.0123456789abcdef`: kubeadm token, this needs to be the same as the one used by kube master

* Use pod definition in samples/ to create sample pod, so you can try out connection

### All-In-One Ubuntu Linux

* Install necessary packages
  * Copy terraform/files/pkg-install.sh to the Ubuntu Linux box
  * Run it as sudo
    ```
    sudo sh /path/to/pkg-install.sh
    ```
* Setup the box as kube master
  * Copy script/ to the Ubuntu Linux box which has an IPv6 address
  * Go to the script/ directory, run
    ```
    sudo -i
    sh master-setup.sh k6-aio fd00:1234::/110 fd00:5678::/110 123456.0123456789abcdef
    ```
    * `k6-aio`: any name good for host name
    * `fd00:1234::/110`: pod CIDR
    * `fd00:5678::/110`: service CIDR
    * `123456.0123456789abcdef`: kubeadm token, since it's all-in-one deployment so there is no node, hence this value doesn't matter
* Untaint the host so pods can be scheduled to this host:
  ```
  kubectl taint nodes --all node-role.kubernetes.io/master-
  ```
* Use pod definition in samples/ to create sample pod, so you can try out connection

Quick Note:
* VPC with IPv6 enabled
* All VMs need to have source/dest check disabled
* wide open k8s-cluster SG within all kube EC2s

TODO
* Fine tuned k8s-cluster SG, or split it into kube-master and kube-node SG
* Replace script with configuration manage tool, Ansible is good for this as it's serverless, but Ansible is really a bad CMS tool
* More cloud support
  * Openstack, 
  * GCE,
  * Azure
* Development support: 
  * vagrant
  * All-in-one Mac
