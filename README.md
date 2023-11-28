# qtap-operator

A kubernetes operator to simplify routing outbound traffic through Qpoint's 3rd-party API Gateway

## Install

Helm

```text
helm install qtap-operator qpoint/qtap-operator --namespace qpoint
```

Manual


The pre-built Docker container can be found at us-docker.pkg.dev/qpoint-edge/public/kubernetes-qtap-operator and uses the tag for the release <https://github.com/qpoint-io/kubernetes-qtap-operator/releases>. See <https://github.com/qpoint-io/helm-charts/blob/main/charts/qtap-operator/templates/deployment.yaml> for an example of a Deployment.

## Configure Egress

__Option 1:__ Namespace label

```text
kubectl label namespace <namespace> qpoint-egress=enabled
```

__Option 2:__ Pod annotation

```text
apiVersion: v1
kind: Pod
metadata:
  name: hello-world
  annotations:
    qpoint.io/egress: enabled
```

The order of precedence is that a pod annotation can override a namespace label. For example the following would enable for a namespace but disable for a pod.

```text
kubectl label namespace <namespace> qpoint-egress=enabled
```

```text
apiVersion: v1
kind: Pod
metadata:
  name: hello-world
  annotations:
    qpoint.io/egress: disabled
```

## Local Dev

Bootstrap dev cluster (uses KinD) with live-reloading

```text
make dev
```

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
