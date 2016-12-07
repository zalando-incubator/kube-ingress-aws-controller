package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"flag"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"strconv"
)

var (
	autoScalingGroupName string
	securityGroupName    string
	apiServerBaseURL     string
	pollingInterval      uint
)

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	for _, s := range signals {
		signal.Notify(c, s)
	}
	return c
}

func loadEnviroment() error {
	flag.Usage = usage
	flag.StringVar(&autoScalingGroupName, "auto-scaling-group-name", "", "manually sets the auto scaling group name. "+
		"if empty will try to resolve that using ec2 metadata")
	flag.StringVar(&securityGroupName, "security-group-name", "", "sets the security group for the managed load balancers. "+
		"if not specified the ASG tag `KubernestesCluster` is used to derive the SG as `<cluster>-worker-lb`")
	flag.StringVar(&apiServerBaseURL, "api-server-base-url", "http://127.0.0.1:8001", "sets the kubernetes api server base url. "+
		"if empty will try to use the common proxy url http://127.0.0.1:8001")
	flag.UintVar(&pollingInterval, "polling-interval", 30, "sets the polling interval (in seconds) for ingress resources. "+
		"Defaults to 30 seconds")
	flag.Parse()

	if autoScalingGroupName == "" {
		autoScalingGroupName = os.Getenv("AUTO_SCALING_GROUP_NAME")
	}

	if securityGroupName == "" {
		securityGroupName = os.Getenv("SECURITY_GROUP_NAME")
	}

	if tmp, defined := os.LookupEnv("API_SERVER_BASE_URL"); defined {
		apiServerBaseURL = tmp
	}

	if tmp, defined := os.LookupEnv("POLLING_INTERVAL"); defined {
		interval, err := strconv.Atoi(tmp)
		if err != nil {
			return err
		}
		pollingInterval = uint(interval)
	}

	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "where options can be:")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.Printf("starting %s", os.Args[0])
	var err error
	if err = loadEnviroment(); err != nil {
		log.Fatal(err)
	}

	session := session.Must(session.NewSession())
	awsAdapter, err := aws.NewAdapter(session, autoScalingGroupName, securityGroupName)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("controller manifest:")
	log.Printf("\tKubernetes cluster: %s\n", awsAdapter.ClusterName())
	log.Printf("\tcurrent vpc id: %s\n", awsAdapter.VpcID())
	log.Printf("\tcurrent instance id: %s\n", awsAdapter.InstanceID())
	log.Printf("\tauto scaling group name: %s\n", awsAdapter.AutoScalingGroupName())
	log.Printf("\tsecurity group name: %s\n", awsAdapter.SecurityGroupID())
	log.Printf("\tprivate subnet ids: %s\n", awsAdapter.PrivateSubnetIDs())
	log.Printf("\tpublic subnet ids: %s\n", awsAdapter.PublicSubnetIDs())

	go startPolling(awsAdapter, apiServerBaseURL, pollingInterval)
	<-waitForTerminationSignals(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Printf("terminating %s\n", os.Args[0])
}
