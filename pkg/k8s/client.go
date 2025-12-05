package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset  *kubernetes.Clientset
	config     *rest.Config
	kubeconfig string
}

// NewClient creates a new Kubernetes client with default kubeconfig
func NewClient() (*Client, error) {
	return NewClientWithConfig("")
}

// NewClientWithConfig creates a new Kubernetes client with specified kubeconfig
func NewClientWithConfig(kubeconfigPath string) (*Client, error) {
	config, kubeconfig, err := getKubeConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		clientset:  clientset,
		config:     config,
		kubeconfig: kubeconfig,
	}, nil
}

// GetKubeConfigPath returns the path of the kubeconfig being used
func (c *Client) GetKubeConfigPath() string {
	return c.kubeconfig
}

func getKubeConfig(kubeconfigPath string) (*rest.Config, string, error) {
	// If a specific path is provided, use it
	if kubeconfigPath != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, "", err
		}
		return config, kubeconfigPath, nil
	}

	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, "(in-cluster)", nil
	}

	// Fall back to kubeconfig file
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", err
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, "", err
	}
	return config, kubeconfig, nil
}

func (c *Client) GetConfig() *rest.Config {
	return c.config
}

func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

// ListNamespaces returns all namespace names
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}

// ListDeployments returns all deployment names in a namespace
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]string, error) {
	deployments, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(deployments.Items))
	for _, dep := range deployments.Items {
		names = append(names, dep.Name)
	}
	sort.Strings(names)
	return names, nil
}

// GetDeployment returns a specific deployment
func (c *Client) GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListPods returns all pods for a deployment
func (c *Client) ListPods(ctx context.Context, namespace, deploymentName string) ([]corev1.Pod, error) {
	deployment, err := c.GetDeployment(ctx, namespace, deploymentName)
	if err != nil {
		return nil, err
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

// ListPodNames returns pod names for a deployment
func (c *Client) ListPodNames(ctx context.Context, namespace, deploymentName string) ([]string, error) {
	pods, err := c.ListPods(ctx, namespace, deploymentName)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		status := string(pod.Status.Phase)
		names = append(names, fmt.Sprintf("%s (%s)", pod.Name, status))
	}
	return names, nil
}

// GetPod returns a specific pod
func (c *Client) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	return c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListContainers returns container names in a pod
func (c *Client) ListContainers(ctx context.Context, namespace, podName string) ([]string, error) {
	pod, err := c.GetPod(ctx, namespace, podName)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		names = append(names, container.Name)
	}
	return names, nil
}

// ScaleDeployment scales a deployment to the specified replicas
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	scale, err := c.clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	scale.Spec.Replicas = replicas
	_, err = c.clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	return err
}

// UpdateImage updates the image of a container in a deployment
func (c *Client) UpdateImage(ctx context.Context, namespace, deploymentName, containerName, image string) error {
	deployment, err := c.GetDeployment(ctx, namespace, deploymentName)
	if err != nil {
		return err
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			deployment.Spec.Template.Spec.Containers[i].Image = image
			break
		}
	}

	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

// GetReplicaSets returns replica sets for a deployment
func (c *Client) GetReplicaSets(ctx context.Context, namespace, deploymentName string) ([]appsv1.ReplicaSet, error) {
	deployment, err := c.GetDeployment(ctx, namespace, deploymentName)
	if err != nil {
		return nil, err
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	rsList, err := c.clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return rsList.Items, nil
}

// GetIngresses returns ingresses that may be related to a deployment
func (c *Client) GetIngresses(ctx context.Context, namespace string) ([]networkingv1.Ingress, error) {
	ingresses, err := c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ingresses.Items, nil
}

// SetEnvVar sets an environment variable on a container in a deployment
func (c *Client) SetEnvVar(ctx context.Context, namespace, deploymentName, containerName, key, value string) error {
	deployment, err := c.GetDeployment(ctx, namespace, deploymentName)
	if err != nil {
		return err
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			found := false
			for j, env := range container.Env {
				if env.Name == key {
					deployment.Spec.Template.Spec.Containers[i].Env[j].Value = value
					found = true
					break
				}
			}
			if !found {
				deployment.Spec.Template.Spec.Containers[i].Env = append(
					deployment.Spec.Template.Spec.Containers[i].Env,
					corev1.EnvVar{Name: key, Value: value},
				)
			}
			break
		}
	}

	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

// GetEnvVars returns environment variables for a container in a deployment
func (c *Client) GetEnvVars(ctx context.Context, namespace, deploymentName, containerName string) ([]corev1.EnvVar, error) {
	deployment, err := c.GetDeployment(ctx, namespace, deploymentName)
	if err != nil {
		return nil, err
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Env, nil
		}
	}

	return nil, fmt.Errorf("container %s not found in deployment %s", containerName, deploymentName)
}

// RollbackDeployment rolls back a deployment to a previous revision
func (c *Client) RollbackDeployment(ctx context.Context, namespace, name string, revision int64) error {
	// Get the deployment
	deployment, err := c.GetDeployment(ctx, namespace, name)
	if err != nil {
		return err
	}

	// Get replica sets
	rsList, err := c.GetReplicaSets(ctx, namespace, name)
	if err != nil {
		return err
	}

	// Find the replica set with the target revision
	var targetRS *appsv1.ReplicaSet
	for i := range rsList {
		rs := &rsList[i]
		if rs.Annotations["deployment.kubernetes.io/revision"] == fmt.Sprintf("%d", revision) {
			targetRS = rs
			break
		}
	}

	if targetRS == nil {
		return fmt.Errorf("revision %d not found", revision)
	}

	// Update deployment with the pod template from the target replica set
	deployment.Spec.Template = targetRS.Spec.Template
	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}
