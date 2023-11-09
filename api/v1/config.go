package v1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const ANNOTATIONS_CONFIGMAP = "qtap-operator-default-pod-annotations-configmap"

type Config struct {
	Namespace string
	Enabled   bool
	InjectCa  bool

	apiClient   client.Client
	annotations map[string]string
}

func (c *Config) Init(ctx context.Context, pod *corev1.Pod) error {
	// check to see if an annotation is set on the pod to enable egress
	egress, exists := pod.Annotations["qpoint.io/egress"]
	if exists && egress == "enabled" {
		c.Enabled = true
	}

	// if we're not enabled yet, let's check the namespace
	if !c.Enabled {
		namespace := &corev1.Namespace{}
		if err := c.apiClient.Get(ctx, client.ObjectKey{Name: c.Namespace}, namespace); err != nil {
			return fmt.Errorf("fetching namespace '%s' from the api: %w", c.Namespace, err)
		}

		// if the namespace is labeled, then we enable
		if namespace.Labels["qpoint-egress"] == "enabled" {
			c.Enabled = true
		}
	}

	// if we're enabled
	if c.Enabled {

		// first, what's our current namespace
		thisNamespace, err := getCurrentNamespace()
		if err != nil {
			return fmt.Errorf("fetching this current namespace: %w", err)
		}

		// let's fetch the default settings in the configmap
		configMap := &corev1.ConfigMap{}
		if err := c.apiClient.Get(ctx, client.ObjectKey{Name: ANNOTATIONS_CONFIGMAP, Namespace: thisNamespace}, configMap); err != nil {
			return fmt.Errorf("fetching configmap '%s' at namespace '%s' from the api: %w", ANNOTATIONS_CONFIGMAP, thisNamespace, err)
		}

		// unmarshal the data as yaml
		defaultAnnotations := make(map[string]string)
		if err := yaml.Unmarshal([]byte(configMap.Data["annotations.yaml"]), &defaultAnnotations); err != nil {
			return fmt.Errorf("marshaling the configmap data as yaml: %w", err)
		}

		// let's apply the default annotations to the pod (for transparency to the admin)
		for key, value := range defaultAnnotations {
			if _, exists := pod.Annotations[key]; !exists {
				pod.Annotations[key] = value
			}
		}

		// and store a direct reference to the annotations for config
		c.annotations = pod.Annotations
	}

	// determine if we should inject the certificate authority
	if c.Get("inject-ca") == "true" {
		c.InjectCa = true
	}

	return nil
}

func (c *Config) Get(key string) string {
	return c.annotations[fmt.Sprintf("qpoint.io/%s", key)]
}

func getCurrentNamespace() (string, error) {
	nsFile := filepath.Join("/var/run/secrets/kubernetes.io/serviceaccount", "namespace")
	namespace, err := os.ReadFile(nsFile)
	if err != nil {
		return "", err
	}
	return string(namespace), nil
}
