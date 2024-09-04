// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package kubeutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
)

func GetClientset() (*kubernetes.Clientset, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	return clientset, nil
}

func WaitForPodIP(ctx context.Context, clientset kubernetes.Interface, namespace string, name string) (string, error) {
	watch, err := clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{Name: name}))
	if err != nil {
		return "", fmt.Errorf("failed to watch pod: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("failed to wait for pod IP before timeout: %w", ctx.Err())
		case event := <-watch.ResultChan():
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				return "", fmt.Errorf("unexpected type: %T", event.Object)
			}
			if pod.Status.PodIP != "" {
				return pod.Status.PodIP, nil
			}
		}
	}
}

// enableLocalForwardForPod enables traffic forwarding from a local
// port to the specified pod with client-go. The returned stop channel
// should be closed to stop the port forwarding.
func EnableLocalForwardForPod(kubeConfig *restclient.Config, namespace string, name string, port int, out, errOut io.Writer) (uint16, chan struct{}, error) {
	log.Printf("Forwarding traffic from a local port to port %d of pod %s.%s via the Kube API", port, namespace, name)

	transport, upgrader, err := spdy.RoundTripperFor(kubeConfig)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{
			Transport: transport,
		},
		http.MethodPost,
		&url.URL{
			Scheme: "https",
			Path:   fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, name),
			Host:   strings.TrimPrefix(kubeConfig.Host, "https://"),
		},
	)
	ports := []string{fmt.Sprintf("0:%d", port)}

	// Need to specify 127.0.0.1 to ensure that forwarding is only
	// attempted for the ipv4 address of the pod. By default, kind is
	// deployed with only ipv4, and attempting to connect to a pod
	// with ipv6 will fail.
	// TODO(marun) This should no longer be required
	addresses := []string{"127.0.0.1"}

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	forwarder, err := portforward.NewOnAddresses(dialer, addresses, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create forwarder: %w", err)
	}

	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			// TODO(marun) Need better error handling here? Or is ok for test-only usage?
			panic(err)
		}
	}()

	<-readyChan // Wait for port forwarding to be ready

	// Retrieve the dynamically allocated local port
	forwardedPorts, err := forwarder.GetPorts()
	if err != nil {
		close(stopChan)
		return 0, nil, fmt.Errorf("failed to get forwarded ports: %w", err)
	}
	if len(forwardedPorts) == 0 {
		close(stopChan)
		return 0, nil, fmt.Errorf("failed to find at least one forwarded port: %w", err)
	}
	return forwardedPorts[0].Local, stopChan, nil
}

func WaitForPodStatus(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace string,
	name string,
	acceptable func(*corev1.PodStatus) bool,
) error {
	watch, err := clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{Name: name}))
	if err != nil {
		return fmt.Errorf("failed to initiate watch of pod %s/%s: %w", namespace, name, err)
	}

	for {
		select {
		case event := <-watch.ResultChan():
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			if acceptable(&pod.Status) {
				return nil
			}
		case <-ctx.Done():
			return errors.New("timeout waiting for pod readiness")
		}
	}
}

func PodIsRunning(status *corev1.PodStatus) bool {
	if status.Phase != corev1.PodRunning {
		return false
	}

	for _, containerStatus := range status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	return true
}

func PodHasTerminated(status *corev1.PodStatus) bool {
	return status.Phase == corev1.PodSucceeded || status.Phase == corev1.PodFailed
}
