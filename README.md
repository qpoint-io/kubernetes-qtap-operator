# qtap-operator
A kubernetes operator to simplify routing outbound traffic through Qpoint's 3rd-party API Gateway

## Install

Helm

```
todo
```

Manual

```
todo
```

## Configure Egress

__Option 1:__ Namespace label

```
kubectl label namespace <namespace> qpoint-egress=enabled
```

__Option 2:__ Pod annotation

```
apiVersion: v1
kind: Pod
metadata:
  name: hello-world
  annotations:
    qpoint.io/egress: enabled
```

## Local Dev

Bootstrap dev cluster (uses KinD) with live-reloading

```
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
