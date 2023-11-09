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

__Option 2:__ Pod annotation

## Local Dev

### Setup

1. Start a kind cluster

```
make kind-create
```

2. Provision cert-manager

```
make kind-cert-manager
```

3. Build the image

```
make docker-build
```

4. Load the image into the cluster

```
make kind-upload
```

5. Deploy

```
make deploy
```

6. Cleanup (when done)

```
make kind-delete
```

### Develop

1. Write code

2. Build image

```
make docker-build
```

3. Load the image into the cluster

```
make kind-upload
```

4. Rollout

```
make kind-rollout
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
