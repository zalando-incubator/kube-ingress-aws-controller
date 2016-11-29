package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"time"
)

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	for _, s := range signals {
		signal.Notify(c, s)
	}
	return c
}

func updateAwsFromIngress(quit chan os.Signal, p client.ConfigProvider, autoScalingGroupName string) {
	for {
		select {
		case <-quit:
			return
		default:
			lbs, err := aws.GetLoadBalancers(p, autoScalingGroupName)
			if err != nil {
				panic(err)
			}
			fmt.Println(lbs)
			time.Sleep(time.Second * 30)
		}
	}
}

func main() {
	os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	session := session.Must(session.NewSession())

	asg, err := aws.GetAutoScalingGroupName(session)
	if err != nil {
		panic(err)
	}

	log.Printf("Using %q as the base auto scaling group\n", asg)
	quit := waitForTerminationSignals(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	updateAwsFromIngress(quit, session, asg)

	log.Println("Terminating kube-ingress-aws-controller")
}
