// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/utils/pointer"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/bootstrapmonitor"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
)

func TestE2E(t *testing.T) {
	ginkgo.RunSpecs(t, "bootstrap test suite")
}

const (
	nodeImage          = "localhost:5001/avalanchego"
	latestNodeImage    = nodeImage + ":latest"
	monitorImage       = "localhost:5001/bootstrap-monitor"
	latestMonitorImage = monitorImage + ":latest"

	initContainerName    = "init"
	monitorContainerName = "monitor"
	nodeContainerName    = "node"

	volumeSize = "128Mi"
	volumeName = "data"

	dataDir     = "/data"
	nodeDataDir = dataDir + "/node" // Use a subdirectory of the data path so that os.RemoveAll can be used when starting a new test
)

var (
	skipNodeImageBuild    bool
	skipMonitorImageBuild bool
)

func init() {
	flag.BoolVar(
		&skipNodeImageBuild,
		"skip-node-image-build",
		false,
		"whether to skip building the avalanchego image",
	)
	flag.BoolVar(
		&skipMonitorImageBuild,
		"skip-monitor-image-build",
		false,
		"whether to skip building the bootstrap-monitor image",
	)
}

var _ = ginkgo.Describe("[Bootstrap Tester]", func() {
	const ()

	ginkgo.It("should support continuous testing of node bootstrap", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)

		if skipNodeImageBuild {
			tc.Outf("{{yellow}}skipping build of avalanchego image{{/}}\n")
		} else {
			ginkgo.By("Building the avalanchego image")
			buildNodeImage(tc, nodeImage, false /* forceNewHash */)
		}

		if skipMonitorImageBuild {
			tc.Outf("{{yellow}}skipping build of bootstrap-monitor image{{/}}\n")
		} else {
			ginkgo.By("Building the bootstrap-monitor image")
			buildImage(tc, monitorImage, false /* forceNewHash */, "build_bootstrap_monitor_image.sh")
		}

		ginkgo.By("Configuring a kubernetes client")
		kubeconfigPath := os.Getenv("KUBECONFIG")
		kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		require.NoError(err)
		clientset, err := kubernetes.NewForConfig(kubeConfig)
		require.NoError(err)

		// TODO(marun) Consider optionally deleting the namespace after the test
		ginkgo.By("Creating a kube namespace to ensure isolation between test runs")
		createdNamespace, err := clientset.CoreV1().Namespaces().Create(tc.DefaultContext(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bootstrap-test-e2e-",
			},
		}, metav1.CreateOptions{})
		require.NoError(err)
		namespace := createdNamespace.Name

		ginkgo.By("Creating a node to bootstrap from")
		nodeStatefulSet := createNode(tc, clientset, namespace)
		nodePodName := nodeStatefulSet.Name + "-0"
		waitForPodCondition(tc, clientset, namespace, nodePodName, corev1.PodReady)
		bootstrapID := waitForNodeHealthy(tc, kubeConfig, namespace, nodePodName)
		pod, err := clientset.CoreV1().Pods(namespace).Get(tc.DefaultContext(), nodePodName, metav1.GetOptions{})
		require.NoError(err)
		bootstrapIP := pod.Status.PodIP

		ginkgo.By("Creating a node that will bootstrap from the first node")
		bootstrapStatefulSet := createBootstrapTester(tc, clientset, namespace, bootstrapIP, bootstrapID)
		bootstrapPodName := bootstrapStatefulSet.Name + "-0"

		ginkgo.By("Waiting for the pod image to be updated to include an image digest")
		var containerImage string
		require.Eventually(func() bool {
			image, err := bootstrapmonitor.GetContainerImage(tc.DefaultContext(), clientset, namespace, bootstrapPodName, nodeContainerName)
			if err != nil {
				tc.Outf("Error determining image used by the %q container of pod %s.%s: %v \n", nodeContainerName, namespace, bootstrapPodName, err)
				return false
			}
			if strings.Contains(image, "sha256") {
				containerImage = image
				return true
			}
			return false
		}, e2e.DefaultTimeout, e2e.DefaultPollingInterval)

		ginkgo.By("Waiting for the init container to report the start of a bootstrap test")
		waitForPodCondition(tc, clientset, namespace, bootstrapPodName, corev1.PodInitialized)
		bootstrapStartingMessage := bootstrapmonitor.BootstrapStartingMessage(containerImage)
		waitForLogOutput(tc, clientset, namespace, bootstrapPodName, initContainerName, bootstrapStartingMessage)

		ginkgo.By("Waiting for the pod to report readiness")
		waitForPodCondition(tc, clientset, namespace, bootstrapPodName, corev1.PodReady)

		ginkgo.By("Waiting for the monitor container to report the success of the bootstrap test")
		bootstrapSucceededMessage := bootstrapmonitor.BootstrapSucceededMessage(containerImage)
		waitForLogOutput(tc, clientset, namespace, bootstrapPodName, monitorContainerName, bootstrapSucceededMessage)
		_ = waitForNodeHealthy(tc, kubeConfig, namespace, nodePodName)

		ginkgo.By("Checking that bootstrap testing is resumed when a pod is rescheduled")
		// Retrieve the UID of the pod pre-deletion
		pod, err = clientset.CoreV1().Pods(namespace).Get(tc.DefaultContext(), bootstrapPodName, metav1.GetOptions{})
		require.NoError(err)
		podUID := pod.UID
		require.NoError(clientset.CoreV1().Pods(namespace).Delete(tc.DefaultContext(), bootstrapPodName, metav1.DeleteOptions{}))
		// Wait for the pod to be recreated with a new UID
		require.Eventually(func() bool {
			pod, err := clientset.CoreV1().Pods(namespace).Get(tc.DefaultContext(), bootstrapPodName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false
			}
			if err != nil {
				tc.Outf("Error getting pod %s.%s: %v\n", namespace, bootstrapPodName, err)
				return false
			}
			return pod.UID != podUID
		}, e2e.DefaultTimeout, e2e.DefaultPollingInterval)
		waitForPodCondition(tc, clientset, namespace, bootstrapPodName, corev1.PodInitialized)
		bootstrapResumingMessage := bootstrapmonitor.BootstrapResumingMessage(containerImage)
		waitForLogOutput(tc, clientset, namespace, bootstrapPodName, initContainerName, bootstrapResumingMessage)

		ginkgo.By("Building and pushing a new avalanchego image to prompt the start of a new bootstrap test")
		buildNodeImage(tc, nodeImage, true /* forceNewHash */)

		ginkgo.By("Waiting for the pod image to change")
		require.Eventually(func() bool {
			image, err := bootstrapmonitor.GetContainerImage(tc.DefaultContext(), clientset, namespace, bootstrapPodName, nodeContainerName)
			if err != nil {
				tc.Outf("Error determining image used by the %q container of pod %s.%s: %v \n", nodeContainerName, namespace, bootstrapPodName, err)
				return false
			}
			if len(image) > 0 && image != containerImage {
				containerImage = image
				return true
			}
			return false
		}, e2e.DefaultTimeout, e2e.DefaultPollingInterval)

		ginkgo.By("Waiting for the init container to report the start of a new bootstrap test")
		waitForPodCondition(tc, clientset, namespace, bootstrapPodName, corev1.PodInitialized)
		bootstrapStartingMessage = bootstrapmonitor.BootstrapStartingMessage(containerImage)
		waitForLogOutput(tc, clientset, namespace, bootstrapPodName, initContainerName, bootstrapStartingMessage)
	})
})

func buildNodeImage(tc tests.TestContext, imageName string, forceNewHash bool) {
	buildImage(tc, imageName, forceNewHash, "build_image.sh")
}

func buildImage(tc tests.TestContext, imageName string, forceNewHash bool, scriptName string) {
	require := require.New(tc)

	relativePath := "tests/fixture/bootstrapmonitor/e2e"
	repoRoot, err := getRepoRootPath(relativePath)
	require.NoError(err)

	var args []string
	if forceNewHash {
		// Ensure the build results in a new image hash by preventing use of a cached final stage
		args = append(args, "--no-cache-filter", "execution")
	}

	cmd := exec.CommandContext(
		tc.DefaultContext(),
		filepath.Join(repoRoot, "scripts", scriptName),
		args...,
	) // #nosec G204
	cmd.Env = append(os.Environ(),
		"DOCKER_IMAGE="+imageName,
		"FORCE_TAG_LATEST=1",
		"SKIP_BUILD_RACE=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		require.FailNow("Image build failed: %v\nWith output: %s", err, output)
	}
}

func createNode(tc tests.TestContext, clientset kubernetes.Interface, namespace string) *appsv1.StatefulSet {
	statefulSet := newNodeStatefulSet("avalanchego-node", defaultNodeFlags())
	createdStatefulSet, err := clientset.AppsV1().StatefulSets(namespace).Create(tc.DefaultContext(), statefulSet, metav1.CreateOptions{})
	require.NoError(tc, err)
	return createdStatefulSet
}

func createBootstrapTester(tc tests.TestContext, clientset *kubernetes.Clientset, namespace string, bootstrapIP string, bootstrapNodeID ids.NodeID) *appsv1.StatefulSet {
	flags := defaultNodeFlags()
	flags[config.BootstrapIPsKey] = fmt.Sprintf("%s:%d", bootstrapIP, config.DefaultStakingPort)
	flags[config.BootstrapIDsKey] = bootstrapNodeID.String()

	statefulSet := newNodeStatefulSet("bootstrap-tester", flags)

	// Add the monitor containers to enable continuous bootstrap testing
	initContainer := getMonitorContainer(initContainerName, []string{
		"init",
		"--node-container-name=" + nodeContainerName,
		"--data-dir=" + dataDir,
	})
	initContainer.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: dataDir,
		},
	}
	statefulSet.Spec.Template.Spec.InitContainers = append(statefulSet.Spec.Template.Spec.InitContainers, initContainer)
	monitorContainer := getMonitorContainer(monitorContainerName, []string{
		"wait-for-completion",
		"--node-container-name=" + nodeContainerName,
		"--poll-interval=1s",
	})
	statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers, monitorContainer)

	grantMonitorPermissions(tc, clientset, namespace)

	createdStatefulSet, err := clientset.AppsV1().StatefulSets(namespace).Create(tc.DefaultContext(), statefulSet, metav1.CreateOptions{})
	require.NoError(tc, err)

	return createdStatefulSet
}

func getMonitorContainer(name string, args []string) corev1.Container {
	return corev1.Container{
		Name:    name,
		Image:   latestMonitorImage,
		Command: []string{"./bootstrap-monitor"},
		Args:    args,
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
		},
	}
}

// grantMonitorPermissions grants the permissions required by the bootstrap monitor to the namespace's default service account.
func grantMonitorPermissions(tc tests.TestContext, clientset *kubernetes.Clientset, namespace string) {
	require := require.New(tc)

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "bootstrap-monitor-role-",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "create", "watch", "delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"patch"},
			},
		},
	}
	createdRole, err := clientset.RbacV1().Roles(namespace).Create(tc.DefaultContext(), role, metav1.CreateOptions{})
	require.NoError(err)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "bootstrap-monitor-role-binding-",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     createdRole.Name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err = clientset.RbacV1().RoleBindings(namespace).Create(tc.DefaultContext(), roleBinding, metav1.CreateOptions{})
	require.NoError(err)
}

func newNodeStatefulSet(name string, flags map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    pointer.Int32(1),
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: volumeName,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(volumeSize),
							},
						},
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  nodeContainerName,
							Image: latestNodeImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: config.DefaultHTTPPort,
								},
								{
									Name:          "staker",
									ContainerPort: config.DefaultStakingPort,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeName,
									MountPath: nodeDataDir,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ext/health",
										Port: intstr.FromInt(config.DefaultHTTPPort),
									},
								},
								PeriodSeconds:    1,
								SuccessThreshold: 1,
							},
							Env: StringMapToEnvVarSlice(flags),
						},
					},
				},
			},
		},
	}
}

func getRepoRootPath(suffix string) (string, error) {
	// - When executed via a test binary, the working directory will be wherever
	// the binary is executed from, but scripts should require execution from
	// the repo root.
	//
	// - When executed via ginkgo (nicer for development + supports
	// parallel execution) the working directory will always be the
	// target path (e.g. [repo root]./tests/bootstrap/e2e) and getting the repo
	// root will require stripping the target path suffix.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(cwd, suffix), nil
}

func StringMapToEnvVarSlice(mapping map[string]string) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, len(mapping))
	var i int
	for k, v := range mapping {
		envVars[i] = corev1.EnvVar{
			Name:  envVarName(config.EnvPrefix, k),
			Value: v,
		}
		i++
	}
	return envVars
}

// TODO(marun) Share one implementation with antithesis configuration
func envVarName(prefix string, key string) string {
	// e.g. MY_PREFIX, network-id -> MY_PREFIX_NETWORK_ID
	return strings.ToUpper(prefix + "_" + config.DashesToUnderscores.Replace(key))
}

func getContainerLogs(tc tests.TestContext, clientset kubernetes.Interface, namespace string, podName string, containerName string) (string, error) {
	// Request the logs
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})

	// Stream the logs
	readCloser, err := req.Stream(tc.DefaultContext())
	if err != nil {
		return "", err
	}
	defer readCloser.Close()

	// Marshal the logs into the versions type
	bytes, err := io.ReadAll(readCloser)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func waitForPodCondition(tc tests.TestContext, clientset *kubernetes.Clientset, namespace string, podName string, conditionType corev1.PodConditionType) {
	require.NoError(tc, bootstrapmonitor.WaitForPodStatus(
		tc.DefaultContext(),
		clientset,
		namespace,
		podName,
		func(status *corev1.PodStatus) bool {
			for _, condition := range status.Conditions {
				if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		},
	))
}

func waitForNodeHealthy(tc tests.TestContext, kubeConfig *restclient.Config, namespace string, podName string) ids.NodeID {
	require := require.New(tc)

	ginkgo.By(fmt.Sprintf("Enabling a local forward for pod %s.%s", namespace, podName))
	localPort, localPortStopChan, err := EnableLocalForwardForPod(kubeConfig, namespace, podName, config.DefaultHTTPPort, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	require.NoError(err)
	defer close(localPortStopChan)
	localNodeURI := fmt.Sprintf("http://127.0.0.1:%d", localPort)

	infoClient := info.NewClient(localNodeURI)
	bootstrapNodeID, _, err := infoClient.GetNodeID(tc.DefaultContext())
	require.NoError(err)

	ginkgo.By(fmt.Sprintf("Waiting for pod %s.%s to report a healthy status at %s", namespace, podName, localNodeURI))
	require.Eventually(func() bool {
		healthReply, err := tmpnet.CheckNodeHealth(tc.DefaultContext(), localNodeURI)
		if err != nil {
			tc.Outf("Error checking node health: %v\n", err)
			return false
		}
		return healthReply.Healthy
	}, e2e.DefaultTimeout, e2e.DefaultPollingInterval)

	return bootstrapNodeID
}

func waitForLogOutput(tc tests.TestContext, clientset *kubernetes.Clientset, namespace string, podName string, containerName string, desiredOutput string) {
	require.Eventually(tc, func() bool {
		logs, err := getContainerLogs(tc, clientset, namespace, podName, containerName)
		if err != nil {
			tc.Outf("Error getting container logs: %v\n", err)
			return false
		}
		return strings.Contains(logs, desiredOutput)
	}, e2e.DefaultTimeout, e2e.DefaultPollingInterval)
}

func defaultNodeFlags() map[string]string {
	return map[string]string{
		config.DataDirKey:                nodeDataDir,
		config.NetworkNameKey:            constants.LocalName,
		config.SybilProtectionEnabledKey: "false",
		config.HealthCheckFreqKey:        "500ms", // Ensure rapid detection of a healthy state
		config.LogDisplayLevelKey:        logging.Debug.String(),
		config.LogLevelKey:               logging.Debug.String(),
		config.HTTPHostKey:               "0.0.0.0", // Need to bind to pod IP to ensure kubelet can access the http port for the readiness check
	}
}

// enableLocalForwardForPod enables traffic forwarding from a local
// port to the specified pod with client-go. The returned stop channel
// should be closed to stop the port forwarding.
func EnableLocalForwardForPod(kubeConfig *restclient.Config, namespace string, name string, port int, out, errOut io.Writer) (uint16, chan struct{}, error) {
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

	// Need to specify 127.0.0.1 to ensure that forwarding is only attempted for the ipv4
	// address of the pod. By default, kind is deployed with only ipv4, and attempting to
	// connect to a pod with ipv6 will fail.
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
