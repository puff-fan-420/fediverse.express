package aws

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/CuteAP/fediverse.express/server/srvcommon"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awss "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/oauth2"
)

type AWSInput struct {
	AccessKeyID     string
	SecretAccessKey string
}

type AWS struct{}

func getSession(accessToken string) (*awss.Session, error) {
	at := strings.Split(accessToken, ":")

	if len(at) < 2 {
		return nil, errors.New("error parsing access token")
	}

	return awss.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(at[0], at[1], ""),
		Region:      aws.String("us-east-1"),
	})
}

func (s *AWS) OAuth2() *oauth2.Config {
	return nil
}

func (s *AWS) EnterCredentials() (string, map[string]string) {
	return "Enter the <b>access key ID</b> and <b>secret access key</b> of a <i>programmatic user</i> that has privileges to create and manage AWS EC2 instances and AWS VPC networks. For more information on how to do this, visit <a href='https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html' target='_blank'>AWS's help site</a>. If you're still having trouble, feel free to reach out.<br><br>Note: any instances created will be created in us-east-1 (North Virginia).",
		map[string]string{
			"AccessKeyID":     "Access key ID",
			"SecretAccessKey": "Secret access key",
		}
}

func (s *AWS) ValidateCredentials(ctx *fiber.Ctx, session *session.Session) error {
	i := &AWSInput{}
	err := ctx.BodyParser(i)
	if err != nil {
		return errors.New("error parsing request body")
	}

	if i.AccessKeyID == "" || i.SecretAccessKey == "" || len(i.AccessKeyID) < 16 {
		return errors.New("something was missing")
	}

	_, err = awss.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(i.AccessKeyID, i.SecretAccessKey, ""),
	})
	if err != nil {
		return errors.New("error initializing session. Check your credentials.")
	}

	session.Set("accessToken", i.AccessKeyID+":"+i.SecretAccessKey)
	session.Set("provider", "aws")

	return nil
}

func (s *AWS) CreateSSHKey(token string, sshKey string) (interface{}, error) {
	sess, err := getSession(token)
	if err != nil {
		return nil, err
	}

	ecx := ec2.New(sess)

	eo, err := ecx.ImportKeyPair(&ec2.ImportKeyPairInput{
		KeyName:           aws.String(srvcommon.RandomString(10) + ".fediverse.express"),
		PublicKeyMaterial: []byte(sshKey),
	})
	if err != nil {
		return nil, err
	}

	return eo.KeyName, nil
}

func (s *AWS) CreateServer(token string, sshKey interface{}) (*string, *string, error) {
	// Why do you have to be so insufferably difficult
	sess, err := getSession(token)
	if err != nil {
		return nil, nil, err
	}

	ecx := ec2.New(sess)

	log.Printf("Creating VPC...")
	// create a VPC to attach to the instance
	vx, err := ecx.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/18"),
	})
	if err != nil {
		return nil, nil, err
	}
	vpcid := vx.Vpc.VpcId

	log.Printf("Creating VPC subnet...")
	// create a subnet to attach to the VPC
	sux, err := ecx.CreateSubnet(&ec2.CreateSubnetInput{
		CidrBlock: aws.String("10.0.0.0/18"),
		VpcId:     vpcid,
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Creating internet gateway")
	ix, err := ecx.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Attaching internet gateway to VPC")
	_, err = ecx.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(*ix.InternetGateway.InternetGatewayId),
		VpcId:             aws.String(*vx.Vpc.VpcId),
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Creating route table for VPC")
	rtx, err := ecx.CreateRouteTable(&ec2.CreateRouteTableInput{
		VpcId: aws.String(*vx.Vpc.VpcId),
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Creating route 0.0.0.0/0 -> Internet on route table via internet gateway")
	_, err = ecx.CreateRoute(&ec2.CreateRouteInput{
		RouteTableId:         aws.String(*rtx.RouteTable.RouteTableId),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(*ix.InternetGateway.InternetGatewayId),
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Associating route table with subnet")
	_, err = ecx.AssociateRouteTable(&ec2.AssociateRouteTableInput{
		SubnetId:     aws.String(*sux.Subnet.SubnetId),
		RouteTableId: aws.String(*rtx.RouteTable.RouteTableId),
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Assigning public IP to subnet on launch")
	_, err = ecx.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
		SubnetId: aws.String(*sux.Subnet.SubnetId),
		MapPublicIpOnLaunch: &ec2.AttributeBooleanValue{
			Value: aws.Bool(true),
		},
	})
	if err != nil {
		return nil, nil, err
	}

	// create a security group to attach to the VPC
	sx, err := ecx.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		Description: aws.String("fediverse.express Misskey installation"),
		GroupName:   aws.String("fediverse-express"),
		VpcId:       vpcid,
	})
	if err != nil {
		return nil, nil, err
	}
	sgid := sx.GroupId

	log.Printf("Authorizing security group ingress...")
	// authorize dem ports
	_, err = ecx.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(*sgid),
		IpPermissions: []*ec2.IpPermission{
			// HTTP >:(
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int64(0),
				ToPort:     aws.Int64(65535),
				IpRanges: []*ec2.IpRange{
					{
						CidrIp: aws.String("0.0.0.0/0"),
					},
				},
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("Starting EC2 instance...")
	// run dem instances
	rx, err := ecx.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String("ami-042e8287309f5df03"), // Ubuntu 20.04 AMI in us-east-1, see https://cloud-images.ubuntu.com/locator/ec2/
		InstanceType: aws.String("t3.small"),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					DeleteOnTermination: aws.Bool(true),
					SnapshotId:          aws.String("snap-0c8d535c6dfde4c4a"), // Linked to Ubuntu 20.04 AMI
					VolumeSize:          aws.Int64(30),                        // GBs
				},
			},
		},
		KeyName:          aws.String(*sshKey.(*string)),
		SecurityGroupIds: []*string{aws.String(*sx.GroupId)},
		SubnetId:         sux.Subnet.SubnetId,
	})

	if err != nil {
		return nil, nil, err
	}

	if len(rx.Instances) < 1 {
		return nil, nil, fmt.Errorf("Instance wasn't created")
	}

	ip := rx.Instances[0].PublicIpAddress

	for ip == nil {
		log.Printf("Fetching instance info...")
		x, err := ecx.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("instance-id"),
					Values: []*string{aws.String(*rx.Instances[0].InstanceId)},
				},
			},
		})

		if err != nil {
			return nil, nil, err
		}

		ip = x.Reservations[0].Instances[0].PublicIpAddress

		time.Sleep(2 * time.Second)
	}

	// give the EC2 machine a little more time to boot up
	time.Sleep(5 * time.Second)

	return ip, nil, nil

	// absolutely what the fuck did I just do
}
