package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apisv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const resyncInterval = 1 * time.Minute

// PodInformer is a event handler for Pod events registered to, that builds a local list of valid and relevant pods
// and sends an event to the endpoint channel, triggering a resync of the targets.
func (a *Adapter) PodInformer(ctx context.Context, endpointChan chan<- []string) (err error) {
	podEndpoints := sync.Map{}

	log.Infof("Watching for Pods with labelselector %s in namespace %s", a.cniPodLabelSelector, a.cniPodNamespace)
	factory := informers.NewSharedInformerFactoryWithOptions(a.clientset, resyncInterval, informers.WithNamespace(a.cniPodNamespace),
		informers.WithTweakListOptions(func(options *apisv1.ListOptions) { options.LabelSelector = a.cniPodLabelSelector }))

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
		log.Errorf("Error listing Pods with labelselector %s in namespace %s: %v", a.cniPodNamespace, a.cniPodLabelSelector, err)
		time.Sleep(resyncInterval)
	}
	for _, pod := range podList {
		if !isPodTerminating(pod) && isPodRunning(pod) {
			podEndpoints.Store(pod.Name, pod.Status.PodIP)
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
				if _, isStored := podEndpoints.LoadOrStore(pod.Name, pod.Status.PodIP); isStored {
					return
				}
				log.Infof("New discovered pod: %s IP: %s", pod.Name, pod.Status.PodIP)
				queueEndpoints(&podEndpoints, endpointChan)
			}
		},
	})
	<-ctx.Done()
	return nil
}

func queueEndpoints(podEndpoints *sync.Map, endpointChan chan<- []string) {
	podList := []string{}
	podEndpoints.Range(func(key, value interface{}) bool {
		podList = append(podList, value.(string))
		return true
	})
	sort.StringSlice(podList).Sort()
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
