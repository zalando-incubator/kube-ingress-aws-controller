# Kubernetes Ingress AWS Controller

**WARNING** This is a work in progress

This application runs inside a Kubernetes cluster monitoring changes to the Ingress 
resources and orchestrating AWS Load Balancers accordingly.

# How it works
The controller should be deployed once per cluster. It will use the EC2 instance metadata to find the CloudFormation
stack it belongs to. This value is then used to find the required AWS resources that are attached to each newly created 
Application Load Balancer:
1. The TargetGroup
    Simple lookup of the "Name" tag matching the stack for the controller node
2. The Security Group
    Lookup of the "Name" tag matching the stack for the controller node and the tag "aws:cloudformation:logical-id"
    matching the value "IngressLoadBalancerSecurityGroup"

## Controller Management Tag

kubernetes:application = kube-ingress-aws-controller
