package v1

import (
	_ "embed"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const QTAP_BUNDLE = "qtap-ca-bundle.crt"
const QPOINT_ROOT_CA = "qpoint-qtap-ca.crt"

//go:embed assets/build-ca.sh
var buildCaScript string

//go:embed assets/alpine-cert.pem
var alpineCertPem string

//go:embed assets/fedora-ca-bundle.crt
var fedoraCaBundle string

//go:embed assets/ubuntu-ca-certificates.crt
var ubuntuCaCertificates string

func MutateCaInjection(pod *corev1.Pod, config *Config) error {
	// generate a volume from the configmap
	configMapVolume := corev1.Volume{
		Name: "qtap-ca-bundle-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: QTAP_BUNDLE,
				},
			},
		},
	}

	// ensure volumes has been initialized
	if pod.Spec.Volumes == nil {
		pod.Spec.Volumes = make([]corev1.Volume, 0)
	}

	// add the volumes to the pod
	pod.Spec.Volumes = append(pod.Spec.Volumes, configMapVolume)

	// define the volume mounts
	volumeMounts := []corev1.VolumeMount{}

	// alpine
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "qtap-ca-bundle-volume",
		MountPath: "/etc/ssl/cert.pem",
		SubPath:   "alpine-cert.pem",
	})

	// fedora
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "qtap-ca-bundle-volume",
		MountPath: "/etc/pki/tls/certs/ca-bundle.crt",
		SubPath:   "fedora-ca-bundle.crt",
	})

	// ubuntu
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "qtap-ca-bundle-volume",
		MountPath: "/etc/ssl/certs/ca-certificates.crt",
		SubPath:   "ubuntu-ca-certificates.crt",
	})

	// add mounts to the containers
	for i := range pod.Spec.Containers {
		// ensure volume mounts have been initialized
		if pod.Spec.Containers[i].VolumeMounts == nil {
			pod.Spec.Containers[i].VolumeMounts = make([]corev1.VolumeMount, 0)
		}

		// append
		pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, volumeMounts...)
	}

	return nil
}

func EnsureAssetsInNamespace(config *Config) error {
	// the goal is to ensure this exists already or we'll create it
	qtapCaExists := false

	// qtap ca configmap reference
	qtapCaConfigMap := &corev1.ConfigMap{}
	qtapCaRef := client.ObjectKey{Namespace: config.Namespace, Name: QTAP_BUNDLE}

	// try to load the qtap ca configmap
	if err := config.Client.Get(config.Ctx, qtapCaRef, qtapCaConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("retrieving Qtap CA config map: %w", err)
		}
	} else {
		qtapCaExists = true
	}

	// if the qtap ca bundle already exists, we're gtg
	if qtapCaExists {
		return nil
	}

	// we need to see if we have the qtap ca in the operator namespace
	qpointRootCaConfigMap := &corev1.ConfigMap{}
	qpointRootCaRef := client.ObjectKey{Namespace: config.OperatorNamespace, Name: QPOINT_ROOT_CA}
	if err := config.Client.Get(config.Ctx, qpointRootCaRef, qpointRootCaConfigMap); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("missing configmap for Qpoint Root CA, check instructions")
		}
		return fmt.Errorf("retrieving Qtap CA config map: %w", err)
	}

	// extract the root CA
	qpointRootCa := qpointRootCaConfigMap.Data["ca.crt"]

	// construct the config map
	qtapCaConfigMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      QTAP_BUNDLE,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			"alpine-cert.pem":            fmt.Sprintf("%s%s\n", alpineCertPem, qpointRootCa),
			"fedora-ca-bundle.crt":       fmt.Sprintf("%s%s\n", fedoraCaBundle, qpointRootCa),
			"ubuntu-ca-certificates.crt": fmt.Sprintf("%s%s\n", ubuntuCaCertificates, qpointRootCa),
		},
	}

	// create it
	if err := config.Client.Create(config.Ctx, qtapCaConfigMap); err != nil {
		return fmt.Errorf("creating configmap for Qtap CA bundles: %w", err)
	}

	return nil
}
