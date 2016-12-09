# Kubernetes Ingress AWS Controller

**WARNING** This is a work in progress

This application runs inside a Kubernetes cluster monitoring changes to the Ingress 
resources and orchestrating AWS Load Balancers accordingly.

# How it works
The controller should be deployed once per cluster. It will use the EC2 instance metadata to fins the clusterID it 
belongs to. This value is then used to find the required AWS resources that are attached to each newly created 
Application Load Balancer:
1. The TargetGroup
2. The Security Group

## Controller Management Tag
kubernetes:application = kube-ingress-aws-controller
