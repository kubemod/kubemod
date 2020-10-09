![Build Status][ci-img] [![Go Report Card][goreport-img]][goreport] [![Code Coverage][cov-img]][cov]

# KubeMod

KubeMod is a universal Kubernetes resource mutator.

It allows you to deploy to Kubernetes declarative rules which perform targeted modifications to specific Kubernetes resources at the time
those resources are deployed or updated.

Essentially, KubeMod is a [Dynamic Admission Control operator](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/),
which gives you the power of Kubernetes Mutating Webhooks without the need to develop an admission webhook controller from scratch.

## Installation

KubeMod is an implementation of a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

To install the operator, run:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.5.0/bundle.yaml
```

To upgrade the operator, run:

```bash
# Delete the kubemod certificate generation job in case kubemod has already been installed.
kubectl.exe delete job -l job-name=kubemod-crt-job -n kubemod-system
# Upgrade kubemod operator.
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.5.0/bundle.yaml
```

To uninstall it, run:

```bash
kubectl delete -f https://raw.githubusercontent.com/kubemod/kubemod/v0.5.0/bundle.yaml
```

**Note**: Uninstalling kubemod operator will also remove all your ModRules.

## Getting started

Once KubeMod is installed, you can deploy ModRules which intercept the creation and update of specific resources and perform modifications on them.

For example, here's a `ModRule` which enforces a `securityContext` and adds annotation `my-annotation` to any `Deployment`
resource whose `app` label equals `nginx` and includes a container of any version of nginx that matches `1.14.*`.

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Patch

  match:
    # Match deployments ...
    - query: '$.kind'
      value: 'Deployment'
    # ... with label app=nginx ...
    - query: '$.metadata.labels.app'
      value: 'nginx'
    # ... and at least one container whose image matches nginx:1.14.* ...
    - query: '$.spec.template.spec.containers[*].image'
      regex: 'nginx:1\.14\..*'
    # ... but has no explicit runAsNonRoot security context (note the "negative: true" part):
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
 
 Save the above ModRule to file `my-modrule.yaml` and deploy it to the default namespace of your Kubernetes cluster:
 ```bash
 kubectl apply -f my-modrule.yaml
```

After the ModRule is created, the creation of any nginx Kubernetes Deployment resource in the same namespace will be intercepted by the KubeMod operator and if the Deployment resource matches all the queries in the ModRule's `match` section, the resource will be patched with the `patch` operations
**before** it is actually deployed to Kubernetes.

See more examples of ModRules [here](https://github.com/kubemod/kubemod/tree/master/core/testdata/modrules).

## Motivation and use cases

Ironically, the development of the KubeMod operator was motivated by the proliferation of the Kubernetes Operator pattern itself.

A large number of services and platforms are now being deployed to Kubernetes using bespoke customized operators instead of deploying primitive Kubernetes resources through Helm charts, kustomize or kubectl.

This is all great as the operator pattern encapsulates the complexity of deploying a ton of primitive resources and boils it down to a number of domain-specific custom resources.

But the operator pattern introduces a challenge -- since the operator is a black-box sending primitive resources directly to the Kubernetes API, this makes it impossible to apply additional customizations to those resources prior to or at the time of deployment.

This leads to issues such as:

- https://github.com/elastic/cloud-on-k8s/issues/2328
- https://github.com/jaegertracing/jaeger-operator/issues/1096

With the help of KubeMod ModRules one can alleviate such issues by intercepting the creation of resources and modifying them to perform the necessary modifications.

There are a few typical use cases for using ModRules.

### Sidecar injection

With the help of ModRules, one can dynamically inject arbitrary sidecar containers into Deployments and StatefulSet objects.
The `patch` part of the ModRule is a [Golang template](https://golang.org/pkg/text/template/) which takes the target object as an intrinsic context allowing for powerful declarative rules such as the following one which injects a [Jaeger Agent](https://www.jaegertracing.io/docs/1.19/architecture/#agent) sidecar into any Deployment tagged with annotation `my-inject-annotation` set to `"true"`:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Patch

  match:
    - query: '$.kind'
      value: Deployment
    - query: '$.metadata.annotations["my-inject-annotation"]'
      value: '"true"'
    # Ensure that there isn't already a jaeger-agent container injected in the pod to avoid adding more containers on UPDATE operations.
    - query: '$.spec.template.spec.containers[*].name'
      value: 'jaeger-agent'
      negative: true

  patch:
    - op: add
      # Use -1 to insert the new container at the end of the containers list.
      path: /spec/template/spec/containers/-1
      value: |-
        name: jaeger-agent
        image: jaegertracing/jaeger-agent:1.18.1
        imagePullPolicy: IfNotPresent
        args:
          # Use template context {{ .Target }} and {{ .Namespace }} to extract the name of the deployment and the current namespace.
          - --jaeger.tags=deployment.name={{ .Target.metadata.name }},pod.namespace={{ .Namespace }},pod.id=${POD_ID:},host.ip=${HOST_IP:}
          - --reporter.grpc.host-port=dns:///jaeger-collector-headless.{{ .Namespace }}:14250
          - --reporter.type=grpc
        env:
          - name: POD_ID
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.uid
          - name: HOST_IP
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: status.hostIP
        ports:
        - containerPort: 6832
          name: jg-binary-trft
          protocol: UDP
```

Note the use of ``{{ .Target.metadata.name }}`` in the patch value to dynamically access the name of the deployment being patched and pass it to the Jaeger agent as a tracer tag.

When a patch is evaluated, KubeMod executes the patch value as a [Golang template](https://golang.org/pkg/text/template/) and passes the following intrinsic items to it accessible through the template's context:
* `.Target` - the original object being patched with all its properties.
* `.Namespace` - the namespace of the object.


### Metadata modifications

With the help of ModRules, one can dynamically modify the resources generated by one operator such that another operator can detect those resources.

For example, [Istio's sidecar injection](https://istio.io/latest/docs/setup/additional-setup/sidecar-injection/) is controlled by pod annotation `sidecar.istio.io/inject`. If an operator creates a deployment which we want to explicitly include or exclude from Istio's injection mechanism, we can create a ModRule which modifies that deployment by adding this annotation.

The following ModRule explicitly excludes the Jaeger collector deployment created by the [Jaeger Operator](https://www.jaegertracing.io/docs/1.18/operator/) from Istio injection:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Patch

  match:
    - query: '$.kind'
      value: 'Deployment'

    - query: '$.metadata.labels.app'
      value: 'jaeger'

    - query: '$.metadata.labels["app.kubernetes.io/component"]'
      value: 'collector'

    # Exclude all deployments which already have annotation sidecar.istio.io/inject applied.
    - query: '$.metadata.annotations["sidecar.istio.io/inject"]'
      negative: true
    
  patch:
    # Add Istio annotation to exclude this deployment from Istio injection.
    - op: add
      path: /metadata/annotations/sidecar.istio.io~1inject
      value: '"false"'
```


### Behavior modifications

Here's a typical black-box operator issue which can be fixed with KubeMod: https://github.com/elastic/cloud-on-k8s/issues/2328.

The issue is that the when the Elastic Search operator creates Persistent Volumes, it attaches an `ownerReference` to them such that they are removed after the operator removes the Elastic Search stack of resources.

This makes sense when we plan to dynamically scale Elastic Search up and and down, but it doesn't make sense if we don't plan to scale dynamically, but we do want to keep the PVC such that it can be reused on the next deployment of an Elastic Search stack (see comments [here](https://github.com/elastic/cloud-on-k8s/issues/2328#issuecomment-583254122) and [here](https://github.com/elastic/cloud-on-k8s/issues/2328#issuecomment-650335893).

A solution to this issue would be the following ModRule which simply removes the `ownerReference` from the PVC before it is actually deployed, thus keeping the PVC persist after the stack is removed:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-mod-rule
spec:
  type: Patch

  matches:
    - query: '$.kind'
      value: PersistentVolumeClaim
    - query: '$.metadata.labels["common.k8s.elastic.co/type"]'
      value: elasticsearch

  patch:
    - op: remove
      path: /metadata/ownerReferences/0
```

### Resource rejection

There are two types of ModRules -- `Patch` and `Reject`.

So far all the examples we've seen have been of type `Patch`.

`Reject` ModRules are simpler as they only have a `match` section.
If a resource matches that `match` section of a `Reject` ModRule, it's creation\update will be rejected.
This allows us to develop a system of policy ModRules which enforce certain rules for the namespace they are deployed to.

For example, here's a `Reject` ModRule which rejects the deployment of any `Deployment` or `StatefulSet` resource which does not explicitly require non-root containers:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Reject

  match:
    # Match Deployments and StatefulSets...
    - query: '$.kind'
      values:
        - 'Deployment'
        - 'StatefulSet'
    # ... that have no explicit runAsNonRoot security context:
    - query: "$.spec.template.spec.securityContext.runAsNonRoot == true"
      negative: true
```

### Other

ModRules are not limited to the above use cases, nor are they limited to those Kubernetes resource types.

ModRules can be developed to target any Kubernetes object, including Custom Resource objects.
The `match` section of a ModRule is not limited to metadata -- you can build complex match criteria against any part of the resource object.

## Gotchas

When multiple ModRules match the same object, all of the ModRule patches are executed against the object in an indeterminate order.

This is by design.

Be careful when creating ModRules such that their match criteria and patch sections don't overlap leading to unexpected behavior.

## ModRule specification

**WIP**

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
[goreport-img]: https://goreportcard.com/badge/github.com/kubemod/kubemod
[goreport]: https://goreportcard.com/report/github.com/kubemod/kubemod
[cov-img]: https://codecov.io/gh/kubemod/kubemod/branch/master/graph/badge.svg
[cov]: https://codecov.io/github/kubemod/kubemod/
