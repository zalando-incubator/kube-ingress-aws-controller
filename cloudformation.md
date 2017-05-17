# CloudFormation usage in the Kubernetes Ingress Controller for AWS

The controller uses CloudFormation to manage the resources required for the ingresses. This choice guarantees
transactional creation and deletion of those resources leaving less chances for the system to find itself
in an inconsistent state.

The approach also gives additional customization options like the ones described below.

## Resources

The built-in template creates a set of AWS resources necessary to accept and forward requests from the outside world
into your Kubernetes services.

This is achieved by creating an internet-facing Application Load Balancer (ALB) with 2 default listeners
(HTTP and HTTPS) and a Target Group that is attached to the Auto Scaling Group of the worker nodes.

You can further customize the stack by adding other resources or tags for your specific setup.

## Customization

The controller accepts some flags to customize some basic behavior of the stack. You can specify the health
check path and port used by the Target Group to select healthy nodes.

For more advanced customization you can provide another flag with the path for your own CloudFormation
template. Consider checking the [built-in template](aws/ingress-cf-template.yaml) for a reference implementation.

The controller requires the template to comply with some some basic requirements.

### Input Parameters

The controller will set the values for some parameters when it creates a new stack. These parameters can change the
behavior and set of resources created. Each one of the following parameters should be defined in the template:

| Parameter                               	| Description                                                	| Required 	| Default Value        	|
|-----------------------------------------	|------------------------------------------------------------	|----------	|----------------------	|
| LoadBalancerSecurityGroupParameter      	| The security group ID for the Load Balancer                	| Yes      	| -                    	|
| TargetGroupVPCIDParameter               	| The VPCID for the TargetGroup                              	| Yes      	| -                    	|
| LoadBalancerSchemeParameter             	| The Load Balancer scheme - "internal" or "internet-facing" 	| No       	| internet-facing      	|
| TargetGroupHealthCheckPathParameter     	| The health check path for the TargetGroup                  	| No       	| /kube-system/healthz 	|
| TargetGroupHealthCheckPortParameter     	| The health check port for the TargetGroup                  	| No       	| 9999                 	|
| TargetGroupHealthCheckIntervalParameter 	| The health check polling interval for the TargetGroup      	| No       	| 10 secs              	|
| ListenerCertificateParameter            	| The HTTPS Listener certificate ARN (IAM/ACM)               	| No       	| No HTTPS Listener    	|

### Outputs

The controller requires each stack to provide some outputs, necessary for its operation.
The following table describes those outputs:

| Key Name            	| Description                                                                      	|
|---------------------	|----------------------------------------------------------------------------------	|
| LoadBalancerDNSName 	| DNS name for the Application Load Balancer created by the stack                  	|
| TargetGroupARN      	| The ARN of the Target Group created by the stack and referenced by the listeners 	|
