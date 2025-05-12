//go:build !race

package kubernetes

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
)

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

	t.Run("initial state of five ready pods, a terminating and pending one", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			newPod := p.DeepCopy()
			newPod.Name = fmt.Sprintf("skipper-%d", i)
			newPod.Status.PodIP = fmt.Sprintf("1.1.1.%d", i)
			_, err := client.CoreV1().Pods("kube-system").Create(context.Background(), newPod, metav1.CreateOptions{})
			require.NoError(t, err)
		}

		termPod := p.DeepCopy()
		termPod.Name = "skipper-terminating"
		termPod.Status.PodIP = "9.9.9.9"
		termPod.DeletionTimestamp = &metav1.Time{}
		_, err := client.CoreV1().Pods("kube-system").Create(context.Background(), termPod, metav1.CreateOptions{})
		require.NoError(t, err)

		pendingPod := p.DeepCopy()
		pendingPod.Name = "skipper-pending"
		pendingPod.Status.PodIP = ""
		_, err = client.CoreV1().Pods("kube-system").Create(context.Background(), pendingPod, metav1.CreateOptions{})
		require.NoError(t, err)
	})

	go func() {
		err := a.PodInformer(context.Background(), pods)
		require.NoError(t, err)
	}()

	t.Run("receiving event of 5 pod list", func(t *testing.T) {
		require.Eventually(t, func() bool {
			pod, ok := <-pods
			if !ok {
				return false
			}
			t.Logf("Got pods from channel: %s", pod)
			require.NotContains(t, pod, "9.9.9.9", "terminating pod should not appear in the initial list")
			require.NotContains(t, pod, "", "pending pod should not appear in the initial list")
			return reflect.DeepEqual(pod, []string{"1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4", "1.1.1.5"})
		}, wait.ForeverTestTimeout, 200*time.Millisecond)
		// flush channel
		time.Sleep(time.Second / 2)
		for len(pods) > 0 {
			<-pods
		}
	})

	t.Run("deleting one pod triggers updated list", func(t *testing.T) {
		p, err := client.CoreV1().Pods("kube-system").Get(context.Background(), "skipper-3", metav1.GetOptions{})
		require.NoError(t, err)
		delPod := p.DeepCopy()
		delPod.DeletionTimestamp = &metav1.Time{}
		// fake client doesn't implement the full delete flow, mocking the status change
		_, err = client.CoreV1().Pods("kube-system").UpdateStatus(context.Background(), delPod, metav1.UpdateOptions{})
		require.NoError(t, err)
		require.NoError(t, client.CoreV1().Pods("kube-system").Delete(context.Background(), "skipper-3", metav1.DeleteOptions{}))
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

	t.Run("4 new pods created, updates a terminating and pending one", func(t *testing.T) {
		for i := 6; i <= 9; i++ {
			newPod := p.DeepCopy()
			newPod.Name = fmt.Sprintf("skipper-%d", i)
			newPod.Status.PodIP = fmt.Sprintf("1.1.1.%d", i)
			_, err := client.CoreV1().Pods("kube-system").Create(context.Background(), newPod, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = client.CoreV1().Pods("kube-system").UpdateStatus(context.Background(), newPod, metav1.UpdateOptions{})
			require.NoError(t, err)
		}

		termPod := p.DeepCopy()
		termPod.Name = "skipper-4"
		termPod.Status.PodIP = "1.1.1.4"
		termPod.DeletionTimestamp = &metav1.Time{}
		_, err := client.CoreV1().Pods("kube-system").UpdateStatus(context.Background(), termPod, metav1.UpdateOptions{})
		require.NoError(t, err)

		pendingPod := p.DeepCopy()
		pendingPod.Name = "skipper-pending"
		pendingPod.Status.PodIP = ""
		_, err = client.CoreV1().Pods("kube-system").UpdateStatus(context.Background(), pendingPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	})

	t.Run("receiving new event of 8 pods list", func(t *testing.T) {
		require.Eventually(t, func() bool {
			pod, ok := <-pods
			if !ok {
				return false
			}
			t.Logf("Got pods from channel: %s", pod)
			require.NotContains(t, pod, "9.9.9.9", "terminating pod must not be part of the list")
			require.NotContains(t, pod, "", "pending pod must not be part of the list")
			return reflect.DeepEqual(pod, []string{"1.1.1.1", "1.1.1.2", "1.1.1.5", "1.1.1.6", "1.1.1.7", "1.1.1.8", "1.1.1.9"})
		}, wait.ForeverTestTimeout, 200*time.Millisecond)
	})
}

func TestPodStatuses(t *testing.T) {
	t.Run("pod is in status terminating", func(t *testing.T) {
		require.True(t, isPodTerminating(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{},
			},
		}))
	})
	t.Run("pod is not in status terminating", func(t *testing.T) {
		require.False(t, isPodTerminating(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: nil,
			},
		}))
	})

	t.Run("pod is running when IP assigned", func(t *testing.T) {
		require.True(t, isPodRunning(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "skipper-1",
				Labels: map[string]string{"application": "skipper-ingress"},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
				PodIP:             "1.1.1.1",
			},
		}))
	})
	t.Run("pod is not running when IP not assigned", func(t *testing.T) {
		require.False(t, isPodRunning(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "skipper-1",
				Labels: map[string]string{"application": "skipper-ingress"},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
				PodIP:             "",
			},
		}))
	})
}
