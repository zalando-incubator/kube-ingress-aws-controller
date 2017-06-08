## Prerequisites

This document describes the prerequisites for deploying the Kubernetes Ingress Controller on AWS.
The following is needed:

- an additional security group to allow traffic from the internet to the load balancers. This can be done by using the following cloud formation stack which needs to have the same name of the autoscaling group used for the nodes:
```
Resources:
  IngressLoadBalancerSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: {Ref: 'AWS::StackName'}
      SecurityGroupIngress:
      - {CidrIp: 0.0.0.0/0, FromPort: 80, IpProtocol: tcp, ToPort: 80}
      - {CidrIp: 0.0.0.0/0, FromPort: 443, IpProtocol: tcp, ToPort: 443}
      VpcId: "$VPC_ID"
      Tags:
        - Key: "ClusterID"
          Value: "$CLUSTER_ID"
```

- Make the port used by `skipper` (in this example, port `9999`) accessible from the IP range of the VPC to allow health checking from the AWS Application Load Balancer (ALB).

Please also note that the worker nodes will need the right permission to describe autoscaling groups, create load balancers and so on. The full list of required AWS IAM roles is the following:

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
    "Action": "elasticloadbalancing:DescribeTargetGroups",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DescribeTargetGroups",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DescribeLoadBalancers",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:CreateLoadBalancer",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DeleteLoadBalancer",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DescribeListeners",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:CreateListener",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DeleteListener",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DescribeTags",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:CreateTargetGroup",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "elasticloadbalancingv2:DeleteTargetGroup",
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
    "Action": "ec2:DescribeVpcs",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "ec2:DescribeVpcs",
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
},
{
    "Action": "iam:ListServerCertificates",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "iam:GetServerCertificate",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "cloudformation:Get*",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "cloudformation:Describe*",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "cloudformation:List*",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "cloudformation:Create*",
    "Resource": "*",
    "Effect": "Allow"
},
{
    "Action": "cloudformation:Delete*",
    "Resource": "*",
    "Effect": "Allow"
}
```

The decision of how to grant these roles is out of scope for this document and depends on your setup. Possible options are:

- assigning an AWS IAM Instance Profile with an IAM role including all the above permissions to the nodes of the cluster
- use a setup based on [kube2iam](https://github.com/jtblin/kube2iam) like in [Zalando's Kubernetes setup](https://github.com/zalando-incubator/kubernetes-on-aws).
