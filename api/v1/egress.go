package v1

import (
	corev1 "k8s.io/api/core/v1"
)

func MutateEgress(pod *corev1.Pod, config *Config) error {
	return nil
}
