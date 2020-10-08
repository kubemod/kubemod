![ci-img]

# KubeMod

KubeMod is a universal Kubernetes resource mutator.

It allows you to deploy to your Kubernetes cluster declarative rules which perform targeted modifications to specific Kubernetes resources at the time those resources are
deployed or updated.

Essentially, KubeMod is a [Dynamic Admission Control operator](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/), which gives you the power of Kubernetes Mutating Webhooks without the need to develop a DAC webhook controller from scratch.

## Installation

KubeMod is an implementation of a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

To install/upgrade the operator, run:

```bash
# Delete the kubemod certificate generation job in case kubemod has already been installed.
kubectl.exe delete job -l job-name=kubemod-crt-job -n kubemod-system
# Install/upgrade kubemod operator.
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.4.2/bundle.yaml
```

To uninstall it, run:

```bash
# Delete all kubemod-related resources.
kubectl delete -f https://raw.githubusercontent.com/kubemod/kubemod/v0.4.2/bundle.yaml
```

**Note**: Uninstalling kubemod operator will also remove all your ModRules.

## Getting started

Once KubeMod is deployed, you can create ModRules which monitor for specific resources and perform modifications on them.

For example, here's a `ModRule` which enforces a `securityContext` and adds annotation `my-annotation` to any `Deployment`
resource whose `app` label equals `nginx` and includes a container of any subversion of nginx `1.14`.

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Patch

  matches:
    # Match deployments ...
    - query: '$.kind'
      value: 'Deployment'
    # ... with label app=nginx ...
    - query: '$.metadata.labels.app'
      value: 'nginx'
    # ... and containers whose images match nginx:1.14.* ...
    - query: '$.spec.template.spec.containers[*].image'
      regex: 'nginx:1\.14\..*'
    # ... but have no explicit runAsNonRoot security context (note the "negative: true" part):
    - query: "$.spec.template.spec.securityContext.runAsNonRoot == true"
      negative: true
    
  patch:
    # Add custom annotation.
    - op: add
      path: /metadata/annotations/my-annotation
      value: whatever

    # Enforce non-root securityContext and make nginx run as user 101.
    - op: add
      path: /spec/template/spec/securityContext
      value: |-
        fsGroup: 101
        runAsGroup: 101
        runAsUser: 101
        runAsNonRoot: true
```
 
 Save the above to file `my-modrule.yaml` and run:
 ```bash
 kubectl apply -f my-modrule.yaml
```

This will create ModRule `my-modrule` in the current default namespace.
 
After the ModRule is created, the creation of any nginx Kubernetes Deployment resource in the same namespace will be intercepted by the KubeMod operator and if the Deployment resource matches all the queries in the ModRule's `matches` section, the resource will be patched with the `patch` operations
**before** it is actually deployed to Kubernetes.

See more examples of ModRules [here](https://github.com/kubemod/kubemod/tree/master/core/testdata/modrules).

## Contributing

Contributions are greatly appreciated! Leave [an issue](https://github.com/kubemod/kubemod/issues)
or [create a PR](https://github.com/kubemod/kubemod/compare).

### Development Prerequisites

* kubebuilder (2.3.1) (https://book.kubebuilder.io/quick-start.html)
* kustomize (3.8.1) (https://kubernetes-sigs.github.io/kustomize/installation/binaries/)
* telepresence (https://www.telepresence.io/)
* wire (https://github.com/google/wire)

### Dev cycle

Build the image once:
```bash
make docker-build
```
Then deploy the kubemod operator resources and start telepresence which will swap out the kubemod controller with your local host environment:
```
make deploy
dev-env.sh
```
At this point you can develop/debug kubemod locally.

[ci-img]: https://gitlab.com/kubemod/kubemod/badges/master/pipeline.svg
