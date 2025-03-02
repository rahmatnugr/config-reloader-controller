# config-reloader-controller
A Kubernetes controller that watches ConfigMap and Secret changes and triggers rolling updates on associated Deployments, StatefulSets, and DaemonSets.

## Requirements
- Kubernetes cluster (e.g., Kind)
- Go 1.23+
- Kubebuilder 4.5.1+
- Docker (for containerization)

## Features
- Watches ConfigMaps and Secrets specified in a `ConfigReloader` CR.
- Triggers rolling updates on workloads when ConfigMap/Secret data changes.
- Updates CR status with conditions (`ConfigMapSynced`, `SecretSynced`, `WorkloadUpdated`).

## Installation

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/rahmatnugr/config-reloader-controller.git
   cd config-reloader-controller
   ```

2. **Run Helm install**:
   ```bash
   helm install config-reloader-controller ./charts/config-reloader-controller \
     --set image.repository=ghcr.io/rahmatnugr/config-reloader-controller \
     --set image.tag=latest \
     -n default --create-namespace
   ```

## Usage

1. **Apply Example Resources**:
   ```
   kubectl get pods -n default -l app.kubernetes.io/name=config-reloader-controller
   kubectl apply -f examples/my-app-deployment.yaml
   kubectl apply -f examples/my-configmap.yaml
   kubectl apply -f examples/my-secret.yaml
   kubectl apply -f examples/configreloader.yaml
   ```

2. **Test Config Changes**:
   - Edit `examples/my-configmap.yaml` (change data.key value) and reapply
     ```bash
     kubectl apply -f examples/my-configmap.yaml
     ```

   - Watch Pods restart:
     ```bash
     kubectl get pods -l app=my-app -w
     ```

3. **Check Status**:
   ```bash
   kubectl get configreloader test-config-reloader -o yaml
   ```

## Development

1. **Generate Code/Manifest**:
   ```bash
   make generate
   make manifests
   ```

2. **Build and Install CRD**:
   ```bash
   make
   make install
   ```

3. **Run Locally** (for development):
   ```bash
   make run
   ```

4. **Build and Push Docker Image**:
   ```bash
   make docker-build
   make docker-push

   # or buildx for multi arch build
   make docker-buildx
   ```

5. **Deploy to Cluster**:
   ```bash
   make deploy
   ```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

