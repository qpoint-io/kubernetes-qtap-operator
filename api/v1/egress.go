package v1

import (
	corev1 "k8s.io/api/core/v1"
)

func MutateEgress(pod *corev1.Pod, config *Config) error {
	// create an init container
	initContainer := corev1.Container{
		Name:  "qtap-init",
		Image: "us-docker.pkg.dev/qpoint-edge/public/kubernetes-qtap-init",
		Env:   []corev1.EnvVar{},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
	}

	// append to the list
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, initContainer)

	// gtg
	return nil
}
