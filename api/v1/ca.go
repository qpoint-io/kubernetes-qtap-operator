package v1

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const QTAP_BUNDLE = "qtap-ca-bundle.crt"
const QPOINT_ROOT_CA = "qpoint-qtap-ca.crt"
const DEFAULT_ENDPOINT = "https://api.qpoint.io"

type Registration struct {
	Ca string `json:"ca"`
}

type RegistrationResponse struct {
	Registration Registration `json:"registration"`
}

func FetchRegistration(token string) (*Registration, error) {
	endpoint := os.Getenv("ENDPOINT")
	if endpoint == "" {
		endpoint = DEFAULT_ENDPOINT
	}

	// make the API request
	url := fmt.Sprintf("%s/qtap/registration", endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("initializing request: %w", err)
	}

	// set the bearer token
	req.Header.Set("Authorization", "Bearer "+token)

	// fetch the registration
	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching registration: %w", err)
	}
	defer res.Body.Close()

	// check if the response was successful (status code 200)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, status %d", res.StatusCode)
	}

	// Deserialize the JSON response into the struct
	var registration RegistrationResponse
	err = json.NewDecoder(res.Body).Decode(&registration)
	if err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	// extract just the registration
	return &registration.Registration, nil
}

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
	if err := config.Client.Get(config.Ctx, client.ObjectKey{Namespace: config.OperatorNamespace, Name: QPOINT_ROOT_CA}, qpointRootCaConfigMap); err != nil {
		if apierrors.IsNotFound(err) {
			// the config map wasn't found and so we'll attempt to fetch the CA from the API
			// this involves fetching the token secret for accessing the API
			secret := &corev1.Secret{}
			if err := config.Client.Get(config.Ctx, client.ObjectKey{Name: "token", Namespace: config.OperatorNamespace}, secret); err != nil {
				return fmt.Errorf("fetching secret '%s' at namespace '%s' from the api: %w", "token", config.OperatorNamespace, err)
			}

			tokenBytes, exists := secret.Data["token"]
			if !exists {
				return fmt.Errorf("token not found in secret '%s'", "token")
			}

			// convert the []byte data to a string
			token := string(tokenBytes)

			if registration, err := FetchRegistration(token); err == nil {
				// the data gets set as if the config map was able to be fetched even though it was
				// obtained from the API
				qpointRootCaConfigMap.Data = map[string]string{
					"ca.crt": registration.Ca,
				}
			} else {
				return fmt.Errorf("missing configuration for Qpoint Root CA, check instructions")
			}
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
