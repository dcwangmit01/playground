resource "aws_security_group" "jump-host" {
  description = "security group for jump host"
  vpc_id      = "${aws_vpc.default.id}"

  revoke_rules_on_delete = true

  tags {
    Name = "jump-host"
  }

  # Allow IPv4 ICMP from anywhere
  ingress {
    from_port   = -1
    to_port     = -1
    protocol    = "icmp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Allow IPv6 ICMP from anywhere
  ingress {
    from_port        = -1
    to_port          = -1
    protocol         = "icmpv6"
    ipv6_cidr_blocks = ["::/0"]
  }

  # Allow SSH from anywhere
  ingress {
    from_port        = 22
    to_port          = 22
    protocol         = "tcp"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }
}

resource "aws_instance" "jump-host" {
  ami                         = "${var.instance_ami}"
  instance_type               = "${var.instance_size}"
  key_name                    = "${var.aws-keypair}"
  subnet_id                   = "${aws_subnet.public.*.id[0]}"
  vpc_security_group_ids      = ["${aws_security_group.jump-host.id}"]
  associate_public_ip_address = true

  tags {
    Name = "${format("%s-jump-host", var.env-name)}"
  }
}

resource "aws_eip" "jump-host" {
  instance = "${aws_instance.jump-host.id}"
  vpc      = true
}

resource "null_resource" "change-jump-host-hostname" {
  provisioner "remote-exec" {
    inline = [
      "sudo hostnamectl set-hostname jump-host",
    ]
  }

  connection {
    user = "ubuntu"
    host = "${aws_eip.jump-host.public_ip}"
  }
}
