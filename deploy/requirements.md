## Prerequisites

This document describe the prerequisites for the stup in which `Kubernetes Ingress Controller` is deploy. 
The following is needed: 

- an additional security group to allow traffic from the internet to the load balancers. This can be done by using the following cloud formation stack which needs to have the same name of the autoscaling group used for the nodes:
```
Resources:
  IngressLoadBalancerSecurityGroup:
    Properties:
      GroupDescription: {Ref: 'AWS::StackName'}
      SecurityGroupIngress:
      - {CidrIp: 0.0.0.0/0, FromPort: 80, IpProtocol: tcp, ToPort: 80}
      - {CidrIp: 0.0.0.0/0, FromPort: 443, IpProtocol: tcp, ToPort: 443}
      VpcId: "$VPC_ID"
      Tags:
        - Key: "ClusterID"
          Value: "$CLUSTER_ID"
    Type: AWS::EC2::SecurityGroup
```

- Make the port used by `skipper` (in this example, port `9999`) accessible from the IP range of the VPC to allow health checking from the Application Load Balancer. 

Please also note that the worker nodes will need the right permission to describe autoscaling groups, create load balancers and so on. The full list of roles is the following: 

```
{
    "Action": "autoscaling:DescribeAutoScalingGroups",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "autoscaling:AttachLoadBalancers",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "autoscaling:DetachLoadBalancers",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "autoscaling:DetachLoadBalancerTargetGroups",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "autoscaling:AttachLoadBalancerTargetGroups",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:DescribeLoadBalancers",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:CreateLoadBalancer",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:DeleteLoadBalancer",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:DescribeListeners",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:CreateListener",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:DeleteListener",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:DescribeTags",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:CreateTargetGroup",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancing:DeleteTargetGroup",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "ec2:DescribeInstances",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "ec2:DescribeSubnets",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "ec2:DescribeSecurityGroups",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "ec2:DescribeRouteTables",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "acm:ListCertificates",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "acm:DescribeCertificate",
    "Resource": "*",
    "Effect": "Allow"
}
```

The decision of how to grant this roles is out of scope for this document and depends on your setup. Possible options are: 

- giving the roles to the nodes of the cluster
- use a setup based on [kube2iam](https://github.com/jtblin/kube2iam) like in [Zalando's Kubernetes setup](https://github.com/zalando-incubator/kubernetes-on-aws).
