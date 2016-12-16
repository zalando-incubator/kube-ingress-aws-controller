# Kubernetes Ingress Controller for AWS

**WARNING** This is a work in progress **WARNING**

This ingress controller runs inside a Kubernetes cluster monitoring changes to the Ingress resources and orchestrating 
AWS Load Balancers accordingly.

It should be deployed once per kubernetes cluster. It will use the EC2 instance metadata to find the CloudFormation
stack it belongs to. This value is then used to discover the required AWS resources that are attached to each newly 
created  Application Load Balancer.

## How it works

It continuously polls the API server checking for ingress resources. For each cycle it creates load balancers for new
ingress resources and deletes the load balancers for ingress resources that were also deleted.

### Discovery

On startup it discovers the AWS resources relevant for the controller operations:
 
1. The AutoScalingGroup

    Simple lookup of the ASG which name matches the "aws:autoscaling:groupName" tag from the EC2 instance running the
    controller.

2. The Security Group

    Lookup of the "Name" tag matching the stack for the controller node and the tag "aws:cloudformation:logical-id"
    matching the value "IngressLoadBalancerSecurityGroup"

### Creating load balancers

When the controller learns about new ingress resources if looks up for a Kubernetes Annotation 
`zalando.org/aws-load-balancer-ssl-cert` with the ARN of an existing certificate. For ex.:

```
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61
```

For each unique certificate the controller creates a new Application Load Balancer. The created load balancers accept 
only HTTPS traffic which is forwarded to each worker node in the cluster.

Application Load Balancers created by the controller have a custom Tag marking them as managed load balancers:

    `kubernetes:application: kube-ingress-aws-controller`

They also share the "Name" tag as the other resources from the same CloudFormation stack. The load balancer names are
derived from the UUIDs in the certificate ARNs. Due to the 32 characters limitation, dashes are stripped.
 
### Deleting load balancers

When the controller detects that a managed load balancer for the current cluster doesn't have a matching ingress 
resource anymore, it deletes all the previously created resources.

## Current limitations

Due to the fact that the certificate ARN is used as the unique key to create some of the AWS resources, it's not 
possible to use the same certificate across multiple Kubernetes clusters deployed on the same AWS account.



