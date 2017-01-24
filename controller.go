package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"flag"
	"time"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

var (
	apiServerBaseURL string
	pollingInterval  time.Duration
	healthCheckPath  string
	healthCheckPort  uint
)

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	return c
}

func loadEnviroment() error {
	flag.Usage = usage
	flag.StringVar(&apiServerBaseURL, "api-server-base-url", "", "sets the kubernetes api "+
		"server base url. if empty will try to use the configuration from the running cluster")
	flag.DurationVar(&pollingInterval, "polling-interval", 30*time.Second, "sets the polling interval for "+
		"ingress resources. The flag accepts a value acceptable to time.ParseDuration. Defaults to 30 seconds")
	flag.StringVar(&healthCheckPath, "health-check-path", "/kube-system/healthz", "sets the health check path "+
		"for the created target groups")
	flag.UintVar(&healthCheckPort, "health-check-port", 9999, "sets the health check port for the created "+
		"target groups")
	flag.Parse()

	if tmp, defined := os.LookupEnv("API_SERVER_BASE_URL"); defined {
		apiServerBaseURL = tmp
	}

	if tmp, defined := os.LookupEnv("POLLING_INTERVAL"); defined {
		interval, err := time.ParseDuration(tmp)
		if err != nil || interval <= 0 {
			return err
		}
		pollingInterval = interval
	}

	if healthCheckPort == 0 || healthCheckPort > 1<<16-1 {
		return fmt.Errorf("invalid health check port: %d. please use a valid IP port", healthCheckPort)
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
	var (
		awsAdapter  *aws.Adapter
		kubeAdapter *kubernetes.Adapter
		kubeConfig  *kubernetes.Config
		err         error
	)
	if err = loadEnviroment(); err != nil {
		log.Fatal(err)
	}

	awsAdapter, err = aws.NewAdapter(healthCheckPath, uint16(healthCheckPort))
	if err != nil {
		log.Fatal(err)
	}

	if apiServerBaseURL == "" {
		kubeConfig, err = kubernetes.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		kubeConfig = kubernetes.InsecureConfig(apiServerBaseURL)
	}
	kubeAdapter, err = kubernetes.NewAdapter(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("controller manifest:")
	log.Printf("\tkubernetes API server: %s\n", apiServerBaseURL)
	log.Printf("\tCluster ID: %s\n", awsAdapter.ClusterID())
	log.Printf("\tvpc id: %s\n", awsAdapter.VpcID())
	log.Printf("\tinstance id: %s\n", awsAdapter.InstanceID())
	log.Printf("\tauto scaling group name: %s\n", awsAdapter.AutoScalingGroupName())
	log.Printf("\tsecurity group id: %s\n", awsAdapter.SecurityGroupID())
	log.Printf("\tprivate subnet ids: %s\n", awsAdapter.PrivateSubnetIDs())
	log.Printf("\tpublic subnet ids: %s\n", awsAdapter.PublicSubnetIDs())

	go startPolling(awsAdapter, kubeAdapter, pollingInterval)
	<-waitForTerminationSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Printf("terminating %s\n", os.Args[0])
}
