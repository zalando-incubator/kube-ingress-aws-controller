package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"flag"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
    "github.com/zalando-incubator/kube-ingress-aws-controller/k8s"
)

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	for _, s := range signals {
		signal.Notify(c, s)
	}
	return c
}

func updateAwsFromIngress(p client.ConfigProvider, autoScalingGroupName string) {
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
		time.Sleep(time.Second * 30)
	}
}

var (
	autoScalingGroup string
	apiServer        string
)

func loadEnviroment() {
	flag.Usage = usage
	flag.StringVar(&autoScalingGroup, "auto-scaling-group", "", "manually sets the auto scaling group name. "+
		"if empty will try to resolve that using ec2 metadata")
	flag.StringVar(&apiServer, "api-server", "http://127.0.0.1:8001", "sets the kubernetes api server base url. "+
		"if empty will try to use the common proxy url http://127.0.0.1:8001")
	flag.Parse()

	if autoScalingGroup == "" {
		autoScalingGroup = os.Getenv("AUTO-SCALING-GROUP")
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "where options can be:")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	//os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	loadEnviroment()

	session := session.Must(session.NewSession())

	var err error
	if autoScalingGroup == "" {
		if aws.RunningOnEc2(session) {
			autoScalingGroup, err = aws.GetAutoScalingGroupName(session)
			if err != nil {
				panic(err)
			}
		} else {
			log.Println("not running on EC2. You have to specify the auto scaling group name.")
			usage()
		}
	}

	log.Printf("using %q as the base auto scaling group\n", autoScalingGroup)
	updateAwsFromIngress(session, autoScalingGroup)
	<-waitForTerminationSignals(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Println("terminating kube-ingress-aws-controller")
}
