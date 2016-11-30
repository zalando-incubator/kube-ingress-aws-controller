package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/k8s"
)

func updateAwsFromIngress(p client.ConfigProvider, autoScalingGroupName string, pollInterval uint) {
	for {
		il, err := k8s.ListIngress()
		if err != nil {
			log.Println(err)
		} else {
			fmt.Println(il)
		}

		lbs, err := aws.GetLoadBalancers(p, autoScalingGroupName)
		if err != nil {
			log.Println(err)
		} else {
			fmt.Println(lbs)
		}
		time.Sleep(time.Second * time.Duration(pollInterval))
	}
}
