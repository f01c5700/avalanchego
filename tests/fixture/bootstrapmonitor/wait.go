// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bootstrapmonitor

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

const (
	defaultContextDuration = 30 * time.Second
)

func WaitForCompletion(namespace string, podName string, nodeContainerName string, interval time.Duration) error {
	var (
		clientset       *kubernetes.Clientset
		reportedSuccess bool
		containerImage  string
	)
	err := wait.PollImmediateInfinite(interval, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultContextDuration)
		defer cancel()

		if healthy, err := tmpnet.CheckNodeHealth(ctx, "http://localhost:9650"); err != nil {
			log.Printf("failed to wait for node health: %v", err)
			return false, nil
		} else if !healthy.Healthy {
			return false, nil
		}

		if clientset == nil {
			var err error
			clientset, err = getClientset()
			if err != nil {
				log.Printf("failed to get clientset: %v", err)
				return false, nil
			}
		}

		if len(containerImage) == 0 {
			var err error
			log.Printf("Retrieving pod %s.%s to determine the image of container %q", namespace, podName, nodeContainerName)
			containerImage, err = GetContainerImage(ctx, clientset, namespace, podName, nodeContainerName)
			if err != nil {
				log.Printf("failed to get container image: %v", err)
				return false, nil
			}
			log.Printf("Image for container %q: %s", nodeContainerName, containerImage)
		}

		if !reportedSuccess {
			log.Println(BootstrapSucceededMessage(containerImage))
			reportedSuccess = true
		}

		latestImageID, err := getLatestImageID(ctx, clientset, namespace, containerImage, nodeContainerName)
		if err != nil {
			log.Printf("failed to get latest image id: %v", err)
			return false, nil
		}

		if latestImageID == containerImage {
			log.Printf("Latest image %s has already bootstrapped successfully", latestImageID)
			return false, nil
		}

		if err := setContainerImage(ctx, clientset, namespace, podName, nodeContainerName, latestImageID); err != nil {
			log.Printf("failed to set container image: %v", err)
			return false, nil
		}

		// Statefulset will restart the pod with the new image
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to wait for completion: %w", err)
	}

	// Avoid exiting immediately to avoid container restart before the pod is recreated with the new image
	time.Sleep(5 * time.Minute)
	return nil
}

func BootstrapSucceededMessage(containerImage string) string {
	return "Bootstrap completed successfully for " + containerImage
}
