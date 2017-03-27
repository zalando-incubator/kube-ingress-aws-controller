# Kubernetes Ingress Controller for AWS

**WARNING**: This is work in progress, it's only used in small production use cases and not battletested!

This ingress controller runs inside a Kubernetes cluster monitoring changes to the Ingress resources and orchestrating 
AWS Load Balancers accordingly.

It will use the EC2 instance metadata to find the CloudFormation
stack it belongs to which is then used to discover the required AWS resources that are attached to each newly created Application Load Balancer.

## How it works

The controller continuously polls the API server checking for ingress resources. For each cycle it creates load balancers for new
ingress resources and deletes the load balancers for ingress resources that were also deleted.
The controller *will not* manage the security groups required to allow access from the internet to the load balancers as it assumes that their lifecycle is external to the controller itself. Please refer to the [Deploy](#deploy) section for complete instructions on how to use it.

### Discovery

On startup it discovers the AWS resources relevant for the controller operations:
 
1. The AutoScalingGroup

    Simple lookup of the ASG which name matches the "aws:autoscaling:groupName" tag from the EC2 instance running the
    controller.

2. The Security Group

    Lookup of the "Name" tag matching the stack for the controller node and the tag "aws:cloudformation:logical-id"
    matching the value "IngressLoadBalancerSecurityGroup"

### Creating load balancers

When the controller learns about new ingress resources it uses the host specified to automatically determine the most specific certificate to use. 
An example ingress is the following:

```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: test-app
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          serviceName: test-app-service
          servicePort: main-port
```

The Application Load Balancer that will be created, will have both an HTTP listener and an HTTPS listener. The latter, will use the automatically selected certificate.

Alternatively, the ARN of the desired certificate can be specified with an annotation like in the following example: 

```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          serviceName: test-app-service
          servicePort: main-port
```

Application Load Balancers created by the controller have a custom Tag marking them as managed load balancers:

    kubernetes:application: kube-ingress-aws-controller

They also share the "ClusterID" tag as the other resources from the same CloudFormation stack. The load balancer names
are derived from a truncated SHA1 hash of the certificate ARN, combined with a normalized version of the ClusterID.
Due to the 32 characters limitation, it can be truncated.
 
### Deleting load balancers

When the controller detects that a managed load balancer for the current cluster doesn't have a matching ingress 
resource anymore, it deletes all the previously created resources.

## Building

This project provides a `Makefile` that can be used to build a binary or to build a docker image for it. To build a binary for your current operating system, it is enough to run `make` or `make build.linux` to build an image for Linux. 
To create a docker image, you can execute `make build.docker` instead. 

## Deploy

To deploy the ingress controller, you can use the following yaml as descriptor, after replacing the placeholder for the region:

```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: app-ingress-controller
  namespace: kube-system
  labels:
    application: app-ingress-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      application: app-ingress-controller
  template:
    metadata:
      labels:
        application: app-ingress-controller
    spec:
      containers:
      - name: controller
        image: registry.opensource.zalan.do/teapot/kube-aws-ingress-controller:latest
        env:
        - name: AWS_REGION
          value: $YOUR_REGION
```

The image used can be customized, we provide `registry.opensource.zalan.do/teapot/kube-aws-ingress-controller:latest` as a public usable docker image built from this codebase.

Additionally to the ingress controller, we use [skipper](https://github.com/zalando/skipper) to route the traffic to the application, in a setup that follows what is described [here](https://kubernetes-on-aws.readthedocs.io/en/latest/user-guide/ingress.html).

We deploy `skipper` as a `DaemonSet` using the following yaml: 

```
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: skipper-ingress
  namespace: kube-system
  labels:
    application: skipper-ingress
    version: v0.9.19
    component: ingress
  annotations:
    daemonset.kubernetes.io/strategyType: RollingUpdate
    daemonset.kubernetes.io/maxUnavailable: "1"
spec:
  selector:
    matchLabels:
      application: skipper-ingress
  template:
    metadata:
      name: skipper-ingress
      labels:
        application: skipper-ingress
        version: v0.9.19
        component: ingress
    spec:
      hostNetwork: true
      containers:
      - name: skipper-ingress
        image: registry.opensource.zalan.do/teapot/skipper:v0.9.19
        ports:
        - name: ingress-port
          containerPort: 9999
          hostPort: 9999
        args:
          - "skipper"
          - "-application-log-level=DEBUG"
          - "-kubernetes"
          - "-kubernetes-in-cluster"
          - "-address=:9999"
          - "-proxy-preserve-host"
          - "-serve-host-metrics"
        resources:
          limits:
            cpu: 200m
            memory: 200Mi
          requests:
            cpu: 25m
            memory: 25Mi
```

To complete the setup, the following is also required: 

- create an additional security group to allow traffic from the internet to the load balancers. This can be done by using the following cloud formation stack which needs to have the same name of the autoscaling group used for the nodes:
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


## Contributing

We welcome your contributions, ideas and bug reports via issues and pull requests. Please open an issue first to discuss possible problems and features. 

## Contact 

You can contact the maintainers of the project via email at the address contained in the [MAINTAINERS file](MAINTAINERS).

## License

The MIT License (MIT) Copyright © [2016] Zalando SE, https://tech.zalando.com
Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.