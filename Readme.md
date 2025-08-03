# config-reloader

**A lightweight Kubernetes Operator that reloads Deployments when their mounted ConfigMaps or Secrets change.**

## Overview

`config-reloader` ensures your workloads stay in sync with config changes without needing manual intervention. When a ConfigMap or Secret associated with a workload changes, this operator updates the corresponding Workload's annotation to trigger a rolling restart—just like tools like [Reloader](https://github.com/stakater/Reloader).

---

##  Getting Started

### Prerequisites

* Go `v1.24.0+`
* Docker `17.03+`
* `kubectl` `v1.11.3+`
* Access to a Kubernetes cluster (`v1.11.3+`)

---

## 🛠️ Installation

### 1. Build and push the image

```bash
make docker-build docker-push IMG=<your-registry>/config-reloader:tag
```

> Make sure your environment can access and pull the image from your registry.

### 2. Install the CRDs

```bash
make install
```

### 3. Deploy the operator

```bash
make deploy IMG=<your-registry>/config-reloader:tag
```

> If you encounter RBAC errors, ensure you have cluster-admin privileges.

---

## 📦 Example Usage

Apply a sample custom resource:

```bash
kubectl apply -k config/samples/
```

> ✅ Make sure the sample has valid config to test the behavior.

---

## 🧹 Uninstallation

```bash
# Delete CR instances
kubectl delete -k config/samples/

# Delete CRDs
make uninstall

# Remove controller
make undeploy
```

---

## Distributing the Project

### Option : Install using a YAML bundle

```bash
make build-installer IMG=<your-registry>/config-reloader:tag
kubectl apply -f https://raw.githubusercontent.com/<org>/config-reloader/<branch>/dist/install.yaml
```


> If you update the project, rerun the above command to sync changes.

---

## Contributing

PRs are welcome! Please open an issue first to discuss what you'd like to change.
Run `make help` to explore available targets.

---

## 📖 Learn More

* [Kubebuilder Docs](https://book.kubebuilder.io/)
* [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

---

## 📜 License

Licensed under the [Apache 2.0 License](http://www.apache.org/licenses/LICENSE-2.0).
