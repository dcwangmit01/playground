data "template_file" "k8s-userdata" {
  template = "${file("files/pkg-install.sh")}"
}

resource "aws_security_group" "k8s-cluster" {
  name        = "k8s-cluster"
  description = "security group for kubernetes cluster"
  vpc_id      = "${aws_vpc.default.id}"

  revoke_rules_on_delete = true

  tags {
    Name = "k8s-cluster"
  }

  ingress {
    from_port        = -1
    to_port          = -1
    protocol         = "icmpv6"
    ipv6_cidr_blocks = ["::/0"]
    description      = "Allow IPv6 ICMP from anywhere"
  }

  ingress {
    from_port       = 22
    to_port         = 22
    protocol        = "tcp"
    security_groups = ["${aws_security_group.jump-host.id}"]
    description     = "Allow SSH from jump host"
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    self        = true
    description = "Allow all from other k8s-clusters"
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
    description      = "Allow all egress"
  }
}

resource "aws_instance" "kube-master" {
  count                       = "${var.master_count}"
  ami                         = "${var.instance_ami}"
  instance_type               = "${var.instance_size}"
  key_name                    = "${var.aws-keypair}"
  subnet_id                   = "${aws_subnet.private.*.id[count.index % var.subnet_count]}"
  vpc_security_group_ids      = ["${aws_security_group.k8s-cluster.id}"]
  associate_public_ip_address = false
  source_dest_check           = false
  user_data                   = "${data.template_file.k8s-userdata.rendered}"

  tags {
    Name = "${format("kube-master-%d", count.index)}"
  }
}

resource "aws_instance" "kube-node" {
  count                       = "${var.node_count}"
  ami                         = "${var.instance_ami}"
  instance_type               = "${var.instance_size}"
  key_name                    = "${var.aws-keypair}"
  subnet_id                   = "${aws_subnet.private.*.id[count.index % var.subnet_count]}"
  vpc_security_group_ids      = ["${aws_security_group.k8s-cluster.id}"]
  associate_public_ip_address = false
  source_dest_check           = false
  user_data                   = "${data.template_file.k8s-userdata.rendered}"

  tags {
    Name = "${format("kube-node-%d", count.index)}"
  }
}
