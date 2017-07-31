package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func main() {
	region := flag.String("region", "us-west-1", "Region")
	id := flag.String("id", "", "EC2 Instance ID")
	flag.Parse()

	if *id == "" {
		fmt.Fprintf(os.Stderr, "Need instance ID\n")
		os.Exit(1)
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(*region),
	}))
	svc := ec2.New(sess)

	// get instance details
	descInstResult, err := svc.DescribeInstances(
		&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				&ec2.Filter{
					Name:   aws.String("instance-id"),
					Values: []*string{id},
				},
			},
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Failed to DescribeInstances: %s\n", formatAwsError(err))
		os.Exit(1)
	}

	// make sure there is such an instance
	if len(descInstResult.Reservations) != 1 ||
		len(descInstResult.Reservations[0].Instances) != 1 {
		fmt.Fprintf(os.Stderr, "cannot find EC2 instance with id [%s]\n", *id)
		os.Exit(1)
	}

	// make sure the instance is not terminated or being terminated
	switch *descInstResult.Reservations[0].Instances[0].State.Name {
	case "shutting-down", "terminated":
		fmt.Fprintf(os.Stderr, "cannot terminate a terminated instance\n")
		os.Exit(1)
	}

	// disassociate and delete EIPs
	for _, nic := range descInstResult.Reservations[0].Instances[0].NetworkInterfaces {
		// get more details about the EIP
		descEipResult, err := svc.DescribeAddresses(
			&ec2.DescribeAddressesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("domain"),
						Values: []*string{aws.String("vpc")},
					},
					{
						Name:   aws.String("public-ip"),
						Values: []*string{nic.Association.PublicIp},
					},
				},
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to DescribeAddresses: %s\n", formatAwsError(err))
			os.Exit(1)
		}

		// disassociate EIP with network interface
		_, err = svc.DisassociateAddress(
			&ec2.DisassociateAddressInput{
				AssociationId: descEipResult.Addresses[0].AssociationId,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to DisassociateAddress: %s\n", formatAwsError(err))
			os.Exit(1)
		}

		// delete EIP
		_, err = svc.ReleaseAddress(
			&ec2.ReleaseAddressInput{
				AllocationId: descEipResult.Addresses[0].AllocationId,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to ReleaseAddress: %s\n", formatAwsError(err))
			os.Exit(1)
		}

		// set network interface to delete-on-instance-termination so we don't
		// have to wait for EC2's termination then delete network interface
		_, err = svc.ModifyNetworkInterfaceAttribute(
			&ec2.ModifyNetworkInterfaceAttributeInput{
				Attachment: &ec2.NetworkInterfaceAttachmentChanges{
					AttachmentId:        nic.Attachment.AttachmentId,
					DeleteOnTermination: aws.Bool(true),
				},
				NetworkInterfaceId: nic.NetworkInterfaceId,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to ModifyNetworkInterfaceAttribute: %s\n", formatAwsError(err))
			os.Exit(1)
		}
	}

	// deletr the EC2, the network interface will be deleted at the same time
	_, err = svc.TerminateInstances(
		&ec2.TerminateInstancesInput{
			InstanceIds: []*string{id},
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Failed to TerminateInstances: %s\n", formatAwsError(err))
		os.Exit(1)
	}
}

func formatAwsError(err error) string {
	if aerr, ok := err.(awserr.Error); ok {
		return (aerr.Error())
	} else {
		return (err.Error())
	}
}
