# Deployment guide using Kops and Skipper (http router)

[Kube AWS Ingress Controller](https://github.com/zalando-incubator/kubernetes-on-aws)
creates AWS Application Load Balancer (ALB) that is used to terminate TLS connections and use
[AWS Certificate Manager (ACM)](https://aws.amazon.com/certificate-manager/) or
[AWS Identity and Access Management (IAM)](http://docs.aws.amazon.com/IAM/latest/APIReference/Welcome.html)
certificates. ALBs are used to route traffic to an Ingress http router for example
[skipper](https://github.com/zalando/skipper/), which routes
traffic to Kubernetes services and implements
[advanced features](https://zalando.github.io/skipper/dataclients/kubernetes/)
like green-blue deployments, feature toggles, reate limits,
circuitbreakers, opentracing API, shadow traffic or A/B tests.

In short the major differences from CoreOS ALB Ingress Controller is:

- it uses Cloudformation instead of API calls
- does not have routes limitations from AWS
- automatically finds the best matching ACM and IAM certifacte for your ingress
- you are free to use an http router imlementation of your choice, which can implement more features like green-blue deployments

For this tutorial I assume you have GNU sed installed, if not read
commands with `sed` to modify the files according to the `sed` command
being run. If you are running BSD or MacOS you can use `gsed`.

## Create Kops cluster with cloud labels

Cloud Labels are required to make Kube AWS Ingress Controller work,
because it has to find the AWS Application Load Balancers it manages
by AWS Tags, which are called cloud Labels in Kops.

You have to set some environment variables to choose AZs to deploy to,
your S3 Bucket name for Kops configuration and you Kops cluster name:

```
export AWS_AVAILABILITY_ZONES=eu-central-1b,eu-central-1c
export S3_BUCKET=kops-aws-workshop-<your-name>
export KOPS_CLUSTER_NAME=example.cluster.k8s.local
```


Next, you create the Kops cluster and validate that everything is set up properly.

```
export KOPS_STATE_STORE=s3://${S3_BUCKET}
kops create cluster --name $KOPS_CLUSTER_NAME --zones $AWS_AVAILABILITY_ZONES --cloud-labels kubernetes.io/cluster/$KOPS_CLUSTER_NAME=owned --yes
kops validate cluster
```

### IAM role

This is the effective policy that you need for your EC2 nodes for the
kube-ingress-aws-controller, which we will use:

```
{
  "Effect": "Allow",
  "Action": [
    "acm:ListCertificates",
    "acm:DescribeCertificate",
    "autoscaling:DescribeAutoScalingGroups",
    "autoscaling:AttachLoadBalancers",
    "autoscaling:DetachLoadBalancers",
    "autoscaling:DetachLoadBalancerTargetGroups",
    "autoscaling:AttachLoadBalancerTargetGroups",
    "cloudformation:*",
    "elasticloadbalancing:*",
    "elasticloadbalancingv2:*",
    "ec2:DescribeInstances",
    "ec2:DescribeSubnets",
    "ec2:DescribeSecurityGroups",
    "ec2:DescribeRouteTables",
    "ec2:DescribeVpcs",
    "iam:GetServerCertificate",
    "iam:ListServerCertificates"
  ],
  "Resource": [
    "*"
  ]
}
```

To apply the mentioned policy you have to add [additionalPolicies with kops](https://github.com/kubernetes/kops/blob/master/docs/iam_roles.md) for your cluster, so edit your cluster.

```
kops edit cluster $KOPS_CLUSTER_NAME
```

and add this to your node policy:

```
  additionalPolicies:
    node: |
      [
        {
          "Effect": "Allow",
          "Action": [
            "acm:ListCertificates",
            "acm:DescribeCertificate",
            "autoscaling:DescribeAutoScalingGroups",
            "autoscaling:AttachLoadBalancers",
            "autoscaling:DetachLoadBalancers",
            "autoscaling:DetachLoadBalancerTargetGroups",
            "autoscaling:AttachLoadBalancerTargetGroups",
            "cloudformation:*",
            "elasticloadbalancing:*",
            "elasticloadbalancingv2:*",
            "ec2:DescribeInstances",
            "ec2:DescribeSubnets",
            "ec2:DescribeSecurityGroups",
            "ec2:DescribeRouteTables",
            "ec2:DescribeVpcs",
            "iam:GetServerCertificate",
            "iam:ListServerCertificates"
          ],
          "Resource": ["*"]
        }
      ]
```

After that make sure this was applied to your cluster with:


```
kops update cluster $KOPS_CLUSTER_NAME --yes
kops rolling-update cluster
```


### Security Group for Ingress

To be able to route traffic from ALB to your nodes you need to create
an Amazon EC2 security group with Kubernetes tags, that allow ingress
port 80 and 443 from the internet and everything from ALBs to your
nodes. Tags are used from Kubernetes components to find AWS components
owned by the cluster. We will do with the AWS cli:

```
aws ec2 create-security-group --description ingress.$KOPS_CLUSTER_NAME --group-name ingress.$KOPS_CLUSTER_NAME
aws ec2 describe-security-groups --group-names ingress.$KOPS_CLUSTER_NAME
sgidingress=$(aws ec2 describe-security-groups --filters Name=group-name,Values=ingress.$KOPS_CLUSTER_NAME | jq '.["SecurityGroups"][0]["GroupId"]' -r)
sgidnode=$(aws ec2 describe-security-groups --filters Name=group-name,Values=nodes.$KOPS_CLUSTER_NAME | jq '.["SecurityGroups"][0]["GroupId"]' -r)
aws ec2 authorize-security-group-ingress --group-id $sgidingress --protocol tcp --port 443 --cidr 0.0.0.0/0
aws ec2 authorize-security-group-ingress --group-id $sgidingress --protocol tcp --port 80 --cidr 0.0.0.0/0

aws ec2 authorize-security-group-ingress --group-id $sgidnode --protocol all --port -1 --source-group $sgidingress
aws ec2 create-tags --resources $sgidingress--tags "kubernetes.io/cluster/id=owned" "kubernetes:application=kube-ingress-aws-controller"
```

### AWS Certificate Manager (ACM)

To have TLS termination you can use AWS managed certificates.  If you
are unsure if you have at least one certificate provisioned use the
following command to list ACM certificates:

```
aws acm list-certificates
```

If you have one, you can move on to the next section.

To create an ACM certificate, you have to requset a CSR with a domain name that you own in [route53](https://aws.amazon.com/route53/), for example.org. We will here request one wildcard certificate for example.org:

```
aws acm request-certificate --domain-name *.example.org
```

You will have to successfully do a challenge to show ownership of the
given domain. In most cases you have to click on a link from an e-mail
sent by certificates.amazon.com. E-Mail subject will be `Certificate approval for <example.org>`.

If you did the challenge successfully, you can now check the status of
your certificate. Find the ARN of the new certificate:

```
aws acm list-certificates
```

Describe the certificate and check the Status value:

```
aws acm describe-certificate --certificate-arn arn:aws:acm:<snip> | jq '.["Certificate"]["Status"]'
```

If this is no "ISSUED", your certificate is not valid and you have to fix it.
To resend the CSR validation e-mail, you can use

```
aws acm resend-validation-email
```


### Install components kube-ingress-aws-controller and skipper

kube-ingress-aws-controller will be deployed as deployment with 1
replica, which is ok for production, because it's only configuring
ALBs.

```
REGION=${AWS_AVAILABILITY_ZONES#*,}
REGION=${REGION:0:-1}
sed -i "s/<REGION>/$REGION/" deploy/ingress-controller.yaml
kubectl create -f deploy/ingress-controller.yaml
```

Skipper will be deployed as daemonset:

```
kubectl create -f deploy/skipper.yaml
```

Check, if the installation was successful:

```
kops validate cluster
```

If not and you are sure all steps before were done, please check the logs of the POD, which is not in running state:

```
kubectl -n kube-system get pods -l component=ingress
kubectl -n kube-system logs <podname>
```

### Test deployment

To test features of this ingress stack checkout our [test deployment page](test-deployment.md)
