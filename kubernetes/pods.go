package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const resyncInterval = 1 * time.Minute

func (a *Adapter) storeWatchedPods(pod *corev1.Pod, podEndpoints *sync.Map) {
	for k, v := range pod.Labels {
		selector := fmt.Sprintf("%s=%s", k, v)
		if pod.Namespace == a.cniPodNamespace && selector == a.cniPodLabelSelector {
			log.Infof("New discovered pod: %s IP: %s", pod.Name, pod.Status.PodIP)
			podEndpoints.LoadOrStore(pod.Name, aws.CNIEndpoint{IPAddress: pod.Status.PodIP})
		}
		for _, endpoint := range a.extraCNIEndpoints {
			if endpoint.Namespace == pod.Namespace && endpoint.Podlabel == selector {
				log.Infof("New discovered pod: %s IP: %s", pod.Name, pod.Status.PodIP)
				podEndpoints.LoadOrStore(pod.Name, aws.CNIEndpoint{IPAddress: pod.Status.PodIP, Namespace: pod.Namespace, Podlabel: selector})
			}
		}
	}
}

// PodInformer is a event handler for Pod events registered to, that builds a local list of valid and relevant pods
// and sends an event to the endpoint channel, triggering a resync of the targets.
func (a *Adapter) PodInformer(ctx context.Context, endpointChan chan<- []aws.CNIEndpoint) (err error) {
	podEndpoints := sync.Map{}

	// log.Infof("Watching for Pods with labelselector %s in namespace %s", a.cniPodLabelSelector, a.cniPodNamespace)
	factory := informers.NewSharedInformerFactoryWithOptions(a.clientset, resyncInterval)

	informer := factory.Core().V1().Pods().Informer()
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}

	// list warms the pod cache and verifies whether pods for given specs can be found, preventing to fail silently
	var podList []*corev1.Pod
	for {
		podList, err = factory.Core().V1().Pods().Lister().List(labels.Everything())
		if err == nil && len(podList) > 0 {
			break
		}
		log.Errorf("error listing Pods: %v", err)
		time.Sleep(resyncInterval)
	}
	for _, pod := range podList {
		if !isPodTerminating(pod) && isPodRunning(pod) {
			a.storeWatchedPods(pod, &podEndpoints)
		}
	}
	queueEndpoints(&podEndpoints, endpointChan)

	// delta triggered updates
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newResource interface{}) {
			pod, ok := newResource.(*corev1.Pod)
			if !ok {
				log.Printf("cannot cast object to corev1.Pod %v", newResource)
				return
			}
			switch {

			case isPodTerminating(pod):
				name, exists := podEndpoints.LoadAndDelete(pod.Name)
				if !exists {
					return
				}
				log.Infof("Deleted pod: %s IP: %s", pod.Name, name)
				queueEndpoints(&podEndpoints, endpointChan)

			case isPodRunning(pod):
				if _, isStored := podEndpoints.Load(pod.Name); isStored {
					return
				}
				a.storeWatchedPods(pod, &podEndpoints)
				queueEndpoints(&podEndpoints, endpointChan)

			}
		},
	})
	<-ctx.Done()
	return nil
}

func queueEndpoints(podEndpoints *sync.Map, endpointChan chan<- []aws.CNIEndpoint) {
	podList := []aws.CNIEndpoint{}
	podEndpoints.Range(func(key, value interface{}) bool {
		podList = append(podList, value.(aws.CNIEndpoint))
		return true
	})
	endpointChan <- podList
}

// intermediate states à la kubectl https://github.com/kubernetes/kubernetes/blob/76cdb57ccfbfebc689fbce45f289add8a0562e07/pkg/printers/internalversion/printers.go#L839
func isPodTerminating(p *corev1.Pod) bool {
	return p.DeletionTimestamp != nil
}

func isPodRunning(p *corev1.Pod) bool {
	return p.Status.ContainerStatuses != nil &&
		len(p.Status.ContainerStatuses) > 0 &&
		p.Status.ContainerStatuses[0].State.Running != nil &&
		p.Status.PodIP != ""
}
