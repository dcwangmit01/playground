variable "env-name" {
  description = "environment name"
  type        = "string"
  default     = "k8s-ipv6"
}

variable "subnet_count" {
  default = "3"
}

variable "master_count" {
  default = "1"
}

variable "node_count" {
  default = "2"
}

variable "aws_profile" {
  description = "AWS profile"
  type        = "string"
  default     = ""
}

variable "aws_region" {
  description = "AWS region"
  type        = "string"
  default     = "us-east-1"
}

variable "aws-keypair" {
  description = "AWS keypair that exists in the region"
  type        = "string"
}

variable "ipv4_cidr_block" {
  description = "CIDR block for IPv4"
  type        = "string"
  default     = "10.10.0.0/16"
}

variable "instance_ami" {
  description = "ubuntu-bionic-18.04-amd64-server-20190212.1 in us-east-1"
  type        = "string"
  default     = "ami-0a313d6098716f372"
}

variable "instance_size" {
  type    = "string"
  default = "m4.large"
}

variable "aws-access-key" {
  type = "string"
}

variable "aws-secret-key" {
  type = "string"
}

provider "aws" {
  access_key = "${var.aws-access-key}"
  secret_key = "${var.aws-secret-key}"
  region     = "${var.aws_region}"
}

data "aws_availability_zones" "az" {}

resource "aws_vpc" "default" {
  cidr_block                       = "${var.ipv4_cidr_block}"
  enable_dns_support               = true
  enable_dns_hostnames             = true
  assign_generated_ipv6_cidr_block = true

  tags {
    Name = "${format("%s VPC", var.env-name)}"
  }
}

resource "aws_subnet" "public" {
  count           = "${var.subnet_count}"
  vpc_id          = "${aws_vpc.default.id}"
  cidr_block      = "${cidrsubnet(aws_vpc.default.cidr_block,8,count.index)}"
  ipv6_cidr_block = "${cidrsubnet(aws_vpc.default.ipv6_cidr_block, 8, count.index)}"

  map_public_ip_on_launch         = true
  assign_ipv6_address_on_creation = true

  availability_zone = "${data.aws_availability_zones.az.names[count.index]}"

  tags {
    Name = "${format("public subnet %d for %s VPC", count.index, var.env-name)}"
  }
}

resource "aws_subnet" "private" {
  count           = "${var.subnet_count}"
  vpc_id          = "${aws_vpc.default.id}"
  cidr_block      = "${cidrsubnet(aws_vpc.default.cidr_block,8,count.index+var.subnet_count)}"
  ipv6_cidr_block = "${cidrsubnet(aws_vpc.default.ipv6_cidr_block, 8, count.index+var.subnet_count)}"

  map_public_ip_on_launch         = false
  assign_ipv6_address_on_creation = true

  availability_zone = "${data.aws_availability_zones.az.names[count.index]}"

  tags {
    Name = "${format("private subnet %d for %s VPC", count.index, var.env-name)}"
  }
}

resource "aws_internet_gateway" "default" {
  vpc_id = "${aws_vpc.default.id}"

  tags {
    Name = "${format("IGW for %s", var.env-name)}"
  }
}

resource "aws_egress_only_internet_gateway" "default" {
  vpc_id = "${aws_vpc.default.id}"
}

resource "aws_eip" "nat_gw" {
  count = "${var.subnet_count}"
  vpc   = true

  tags = {
    Name = "${format("NAT GW %d for %s", count.index, var.env-name)}"
  }
}

resource "aws_nat_gateway" "default" {
  count         = "${var.subnet_count}"
  allocation_id = "${element(aws_eip.nat_gw.*.id, count.index)}"
  subnet_id     = "${element(aws_subnet.public.*.id, count.index)}"
}

resource "aws_route_table" "public" {
  vpc_id = "${aws_vpc.default.id}"

  # Route all IPv6 traffic in public subnet to the Internet gateway
  route {
    ipv6_cidr_block = "::/0"
    gateway_id      = "${aws_internet_gateway.default.id}"
  }

  # Route all IPv4 traffic in public subnet to the Internet gateway
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "${aws_internet_gateway.default.id}"
  }

  tags = {
    Name = "public"
  }
}

resource "aws_route_table_association" "public" {
  count          = "${var.subnet_count}"
  subnet_id      = "${element(aws_subnet.public.*.id, count.index)}"
  route_table_id = "${aws_route_table.public.id}"
}

resource "aws_route_table" "private" {
  count  = "${var.subnet_count}"
  vpc_id = "${aws_vpc.default.id}"

  # Route all IPv6 traffic in private subnet to the Egress only ternet gateway
  route {
    ipv6_cidr_block        = "::/0"
    egress_only_gateway_id = "${aws_egress_only_internet_gateway.default.id}"
  }

  # Route all IPv4 traffic in private subnet to the NAT gateway
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = "${element(aws_nat_gateway.default.*.id, count.index)}"
  }

  tags {
    Name = "${format("private-%d", count.index)}"
  }
}

# The route table must be associated with the subnet
resource "aws_route_table_association" "private" {
  count          = "${var.subnet_count}"
  subnet_id      = "${element(aws_subnet.private.*.id, count.index)}"
  route_table_id = "${element(aws_route_table.private.*.id, count.index)}"
}
