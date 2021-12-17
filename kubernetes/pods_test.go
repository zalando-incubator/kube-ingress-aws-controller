// +build !race

package kubernetes

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
)

func kubeconfig() *kubernetes.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	kubeCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		log.Panicf("Can't get kubernetes client: %v", err)
	}
	return clientset
}

func ExampleAdapter_PodInformer() {
	epCh := make(chan []string, 10)
	a := Adapter{
		clientset:           kubeconfig(),
		cniPodNamespace:     "kube-system",
		cniPodLabelSelector: "application=skipper-ingress",
	}
	a.PodInformer(context.TODO(), epCh)
}

// The fake client fails under race tests https://github.com/kubernetes/kubernetes/issues/95372
func TestAdapter_PodInformer(t *testing.T) {
	a := Adapter{
		cniPodNamespace:     "kube-system",
		cniPodLabelSelector: "application=skipper-ingress",
	}

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "skipper-1",
			Labels: map[string]string{"application": "skipper-ingress"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
			PodIP:             "1.1.1.1",
		},
	}
	client := fake.NewSimpleClientset()

	a.clientset = client
	pods := make(chan []string, 10)
	go a.PodInformer(context.TODO(), pods)

	t.Run("creating five pods", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			newPod := p.DeepCopy()
			newPod.Name = fmt.Sprintf("skipper-%d", i)
			newPod.Status.PodIP = fmt.Sprintf("1.1.1.%d", i)
			_, err := client.CoreV1().Pods("kube-system").Create(context.TODO(), newPod, metav1.CreateOptions{})
			require.NoError(t, err)
			time.Sleep(123 * time.Millisecond)
		}
	})
	t.Run("receiving event of 5 pod list", func(t *testing.T) {
		require.Eventually(t, func() bool {
			pod, ok := <-pods
			if !ok {
				return false
			}
			t.Logf("Got pods from channel: %s", pod)
			return reflect.DeepEqual(pod, []string{"1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4", "1.1.1.5"})
		}, wait.ForeverTestTimeout, 200*time.Millisecond)
		// flush channel
		time.Sleep(time.Second / 2)
		for len(pods) > 0 {
			<-pods
		}
	})

	t.Run("deleting one pod triggers updated list", func(t *testing.T) {
		p, err := client.CoreV1().Pods("kube-system").Get(context.TODO(), "skipper-3", metav1.GetOptions{})
		require.NoError(t, err)
		delPod := p.DeepCopy()
		delPod.DeletionTimestamp = &metav1.Time{}
		// fake client doesn't implement the full delete flow, mocking the status change
		_, err = client.CoreV1().Pods("kube-system").UpdateStatus(context.TODO(), delPod, metav1.UpdateOptions{})
		require.NoError(t, err)
		require.NoError(t, client.CoreV1().Pods("kube-system").Delete(context.TODO(), "skipper-3", metav1.DeleteOptions{}))
	})

	t.Run("receiving the update event of only 4 pod list", func(t *testing.T) {
		require.Eventually(t, func() bool {
			pod, ok := <-pods
			if !ok {
				return false
			}
			t.Logf("Got pods from channel: %s", pod)
			return reflect.DeepEqual(pod, []string{"1.1.1.1", "1.1.1.2", "1.1.1.4", "1.1.1.5"})
		}, wait.ForeverTestTimeout, 200*time.Millisecond)
	})

}
