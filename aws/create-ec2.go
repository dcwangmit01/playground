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
	name := flag.String("name", "aws-golang-test", "Name of the instance")
	region := flag.String("region", "us-west-1", "Region")
	ami := flag.String("ami", "ami-73f7da13", "AMI ID")
	flavor := flag.String("flavor", "t2.nano", "Instance type")
	subnet := flag.String("subnet", "subnet-xxxxxxxx", "Subnet ID")
	secGroup := flag.String("security-group", "sg-xxxxxxxx", "Security Group ID")
	keyName := flag.String("key", "hangxie", "Key pair")
	userData := flag.String("user-data", "", "User data")
	nicCount := flag.Int("nic", 2, "Number of network interfaces")
	flag.Parse()

	if *nicCount < 1 {
		fmt.Fprintf(os.Stderr, "Need at least 1 network interface")
		os.Exit(1)
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(*region),
	}))
	svc := ec2.New(sess)

	// create network interface and associate with EIP
	nicSpecs := make([]*ec2.InstanceNetworkInterfaceSpecification, *nicCount)
	for i := 0; i < *nicCount; i++ {
		nicSpecs[i] = &ec2.InstanceNetworkInterfaceSpecification{}
		// create network interface
		createNicResult, err := svc.CreateNetworkInterface(
			&ec2.CreateNetworkInterfaceInput{
				Description: aws.String(fmt.Sprintf("%s-%d", *name, i)),
				Groups:      []*string{aws.String(*secGroup)},
				SubnetId:    aws.String(*subnet),
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to CreateNetworkInterface: %s\n",
				formatAwsError(err))
			os.Exit(1)
		}
		nicSpecs[i].NetworkInterfaceId = createNicResult.NetworkInterface.NetworkInterfaceId
		nicSpecs[i].DeviceIndex = aws.Int64(int64(i))

		// change src/dest check
		_, err = svc.ModifyNetworkInterfaceAttribute(
			&ec2.ModifyNetworkInterfaceAttributeInput{
				NetworkInterfaceId: nicSpecs[i].NetworkInterfaceId,
				SourceDestCheck: &ec2.AttributeBooleanValue{
					Value: aws.Bool(false),
				},
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to ModifyNetworkInterfaceAttribute: %s\n",
				formatAwsError(err))
			os.Exit(1)
		}

		// allocate EIP
		allocAddrResult, err := svc.AllocateAddress(
			&ec2.AllocateAddressInput{Domain: aws.String("vpc")},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to AllocateAddress: %s\n",
				formatAwsError(err))
			os.Exit(1)
		}

		// associate two together
		assocAddrResult, err := svc.AssociateAddress(
			&ec2.AssociateAddressInput{
				NetworkInterfaceId: nicSpecs[i].NetworkInterfaceId,
				AllocationId:       allocAddrResult.AllocationId,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to AssociateAddress: %s\n", formatAwsError(err))
			os.Exit(1)
		}
		fmt.Printf("Association [%s]: [%s] <--> [%s]\n",
			*assocAddrResult.AssociationId,
			*nicSpecs[i].NetworkInterfaceId,
			*allocAddrResult.PublicIp)
	}

	// create EC2
	runInstResult, err := svc.RunInstances(&ec2.RunInstancesInput{
		ImageId:           aws.String(*ami),
		InstanceType:      aws.String(*flavor),
		NetworkInterfaces: nicSpecs,
		KeyName:           aws.String(*keyName),
		MinCount:          aws.Int64(1),
		MaxCount:          aws.Int64(1),
		UserData:          aws.String(*userData),
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to RunInstances: %s\n", formatAwsError(err))
		os.Exit(1)
	}

	// name the EC2
	_, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{runInstResult.Instances[0].InstanceId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(*name),
			},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to CreateTags: %s\n", formatAwsError(err))
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
