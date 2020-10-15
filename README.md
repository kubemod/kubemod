[![Build Status][ci-img]][ci] [![Go Report Card][goreport-img]][goreport] [![Code Coverage][cov-img]][cov]

# KubeMod

KubeMod unlocks the power of [Kubernetes Mutating Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) by introducing `ModRule` - a custom Kubernetes resource which allows you to intercept the creation of any Kubernetes object and apply targeted modifications to it before it is deployed to the cluster.

Use KubeMod to:

* Customize opaque Helm charts and Kubernetes operators
* Build a system of policy rules to reject misbehaving resources
* Develop your own sidecar container injections - no coding required

## Installation

KubeMod is a [Kubernetes operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) deployed to its own namespace `kubemod-system`.

Install it by running:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.5.0/bundle.yaml
```

To upgrade it, run:

```bash
# Delete the kubemod certificate generation job in case kubemod has already been installed.
kubectl.exe delete job -l job-name=kubemod-crt-job -n kubemod-system
# Upgrade kubemod operator.
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.5.0/bundle.yaml
```

To uninstall KubeMod, run:

```bash
kubectl delete -f https://raw.githubusercontent.com/kubemod/kubemod/v0.5.0/bundle.yaml
```

**Note**: Uninstalling KubeMod will also remove all your ModRules.

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
    - select: '$.kind'
      matchValue: 'Deployment'
    # ... with label app = nginx ...
    - select: '$.metadata.labels.app'
      matchValue: 'nginx'
    # ... and at least one container whose image matches nginx:1.14.* ...
    - select: '$.spec.template.spec.containers[*].image'
      matchRegex: 'nginx:1\.14\..*'
    # ... but has no explicit runAsNonRoot security context (note the "negate: true" part):
    - select: "$.spec.template.spec.securityContext.runAsNonRoot == true"
      negate: true
    
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

The creation of KubeMod was motivated by the proliferation of Kubernetes Operators and Helm charts which are sometimes opaque to customizations and lead to runtime issues.

Helm charts and Kubernetes operators greatly simplify the complexity of deploying a ton of primitive resources and reduce it down to a number of configuration values and domain-specific custom resources.

But sometimes this simplicity introduces a challenge -- from a user's perspective, Helm charts and Kubernetes operators are black boxes which can only be controlled through the configuration values the chart/operator developer chose to expose.

Ideally we would not need to control anything more than those configuration values, but in reality this opaqueness leads to issues such as these:

* https://github.com/elastic/cloud-on-k8s/issues/2328
* https://github.com/jaegertracing/jaeger-operator/issues/1096

Oftentimes these issues are showstoppers that render the chart/operator impossible to use for certain use cases.

With the help of KubeMod we can make those charts and operators work for us. Just deploy a cleverly developed ModRule which targets the problematic primitive resource and patch it on the fly at the time it is created.

Here's a number of typical use cases for KubeMod.

(Some of them, such as the **sidecar injection** and **resource rejection**, go beyond the original use case of fixing third-party misbehaving code).


### Behavior modifications

Here's a typical black-box operator issue which can be fixed with KubeMod: https://github.com/elastic/cloud-on-k8s/issues/2328.

The issue is that when the [Elastic Search operator](https://github.com/elastic/cloud-on-k8s) creates Persistent Volume Claims, it attaches an `ownerReference` to them such that they are garbage-collected after the operator removes the Elastic Search stack of resources.

This makes sense when we plan to dynamically scale Elastic Search up and down, but it doesn't make sense if we don't plan to scale dynamically, but we do want to keep the Elastic Search indexes during Elastic Search reinstallation (see comments [here](https://github.com/elastic/cloud-on-k8s/issues/2328#issuecomment-583254122) and [here](https://github.com/elastic/cloud-on-k8s/issues/2328#issuecomment-650335893)).

A solution to this issue would be the following ModRule which simply removes the `ownerReference` from PVCs created by the Elastic Search operator at the time they are deployed, thus excluding those resources from Kubernetes garbage collection:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-mod-rule
spec:
  type: Patch

  matches:
    # Match persistent volume claims ...
    - select: '$.kind'
      matchValue: PersistentVolumeClaim
    # ... created by the elasticsearch operator.
    - select: '$.metadata.labels["common.k8s.elastic.co/type"]'
      matchValue: elasticsearch

  patch:
    # Remove the ownerReference if it exists, thus excluding the resource from Kubernetes garbage collection.
    - op: remove
      path: /metadata/ownerReferences/0
```


### Metadata modifications

With the help of ModRules, one can dynamically modify the resources generated by one operator such that another operator can detect those resources.

For example, [Istio's sidecar injection](https://istio.io/latest/docs/setup/additional-setup/sidecar-injection/) can be controlled by pod annotation `sidecar.istio.io/inject`. If another operator creates a deployment which we want to explicitly exclude from Istio's injection mechanism, we can create a ModRule which modifies that deployment by adding this annotation with value `"false"`.

The following ModRule explicitly excludes the Jaeger collector deployment created by the [Jaeger Operator](https://www.jaegertracing.io/docs/1.18/operator/) from Istio sidecar injection:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Patch

  match:
    # Match deployments ...
    - select: '$.kind'
      matchValue: 'Deployment'

    # ... with label app = jaeger ...
    - select: '$.metadata.labels.app'
      matchValue: 'jaeger'

    # ... and label app.kubernetes.io/component = collector ...
    - select: '$.metadata.labels["app.kubernetes.io/component"]'
      matchValue: 'collector'

    # ... but with and no annotation sidecar.istio.io/inject.
    - select: '$.metadata.annotations["sidecar.istio.io/inject"]'
      negate: true
    
  patch:
    # Add Istio annotation sidecar.istio.io/inject=false to exclude this deployment from Istio injection.
    - op: add
      path: /metadata/annotations/sidecar.istio.io~1inject
      value: '"false"'
```


### Sidecar injection

With the help of ModRules, one can dynamically inject arbitrary sidecar containers into Deployments and StatefulSet resources.
The `patch` part of the ModRule is a [Golang template](https://golang.org/pkg/text/template/) which takes the target resource object as an intrinsic context allowing for powerful declarative rules such as the following one which injects a [Jaeger Agent](https://www.jaegertracing.io/docs/1.19/architecture/#agent) sidecar into any Deployment tagged with annotation `my-inject-annotation` set to `"true"`:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Patch

  match:
    # Match deployments ...
    - select: '$.kind'
      matchValue: Deployment
    # ... with annotation  my-inject-annotation = true ...
    - select: '$.metadata.annotations["my-inject-annotation"]'
      matchValue: '"true"'
    # ... but ensure that there isn't already a jaeger-agent container injected in the pod template to avoid adding more containers on UPDATE operations.
    - select: '$.spec.template.spec.containers[*].name'
      matchValue: 'jaeger-agent'
      negate: true

  patch:
    - op: add
      # Use -1 to insert the new container at the end of the containers list.
      path: /spec/template/spec/containers/-1
      value: |-
        name: jaeger-agent
        image: jaegertracing/jaeger-agent:1.18.1
        imagePullPolicy: IfNotPresent
        args:
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

Note the use of ``{{ .Target.metadata.name }}`` in the patch `value` to dynamically access the name of the deployment being patched and pass it to the Jaeger agent as a tracer tag.

When a patch is evaluated, KubeMod executes the patch value as a [Golang template](https://golang.org/pkg/text/template/) and passes the following intrinsic items accessible through the template's context:
* `.Target` - the original resource object being patched with all its properties.
* `.Namespace` - the namespace of the resource object.


### Resource rejection

There are two types of ModRules -- `Patch` and `Reject`.

All of the examples we've seen so far have been of type `Patch`.

`Reject` ModRules are simpler as they only have a `match` section.

If a resource matches the `match` section of a `Reject` ModRule, its creation/update will be rejected.
This enables the development of a system of policy ModRules which enforce certain security restrictions in the namespace they are deployed to.

For example, here's a `Reject` ModRule which rejects the deployment of any `Deployment` or `StatefulSet` resource that does not explicitly require non-root containers:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: my-modrule
spec:
  type: Reject

  match:
    # Match (thus reject) Deployments and StatefulSets...
    - select: '$.kind'
      matchValues:
        - 'Deployment'
        - 'StatefulSet'
    # ... that have no explicit runAsNonRoot security context.
    - select: "$.spec.template.spec.securityContext.runAsNonRoot == true"
      negate: true
```

### Other

ModRules are not limited to the above use cases, nor are they limited to those Kubernetes resource types.

ModRules can be developed to target any Kubernetes resource object, including Custom Resource objects.
The `match` section of a ModRule is not limited to metadata -- you can build complex match criteria against any part of the resource object.

## Gotchas

When multiple ModRules match the same resource object, all of the ModRule patches are executed against the object in an indeterminate order.

This is by design.

Be careful when creating ModRules such that their match criteria and patch sections don't overlap leading to unexpected behavior.

## ModRule specification

A `ModRule` has a `type`, a required `match` section and an optional `patch` section.

### `type`

Can be one of the following:
- `Patch` - this type of `ModRule` applies patches to objects which match the `match` section. Section `patch` is required for `Patch` ModRules.
- `Reject` - this type of `ModRule` rejects objects which match the `match` section.

### `match`

Section `match` is an array of individual criteria items.

When a new object is deployed to Kubernetes, KubeMod intercepts the operation and attempts to match the new object's definition against all ModRules deployed to the namespace where the object is being deployed.

A ModRule is considered to have a match with the Kubernetes object definition when all criteria items in its `match` section yield a positive match.

A criteria item contains a required `select` expression and optional `matchValue`, `matchValues`, `matchRegex` and `negate` properties.

* `select` - a [JSONPath](https://goessner.net/articles/JsonPath/) expression which, when evaluated against the Kubernetes object definition, yields zero or more values.
* `matchValue` - a string matched against the result of `select`.
* `matchValues` - an array of strings matched against the result of `select`.
* `matchRegex` - a regular expression matched against the result of `select`.

A criteria item is considered positive when its `select` expression yields one or more values and one of the following is true:
 
* No `matchValue`, `matchValues` or `matchRegex` are specified for the criteria item.
* `matchValue` is specified and one or more of the values resulting from `select` exactly matches that value.
* `matchValues` is specified and one or more of the values resulting from `select` exactly matches one or more of the values in `matchValues`.
* `matchRegex` is specified and one or more of the values resulting from `select` matches that regular expression.
 
The result of a criteria item can be inverted by setting its `negate` to `true`.

### `patch`

Section `patch` is an array of [RFC6902 JSON Patch](https://tools.ietf.org/html/rfc6902) operations.

The implementation of JSON Patch used in KubeMod includes the following extensions to RFC6902:

* Negative array indices mean starting at the end of the array.
* Operations which attempt to remove a non-existent path in the JSON object are ignored.

In addition, when a patch is evaluated, KubeMod executes the patch `value` as a [Golang template](https://golang.org/pkg/text/template/) and passes the following intrinsic items accessible through the template's context:

* `.Target` - the original resource object being patched with all its properties.
* `.Namespace` - the namespace of the resource object.


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

[ci-img]: https://github.com/kubemod/kubemod/workflows/Master%20Workflow/badge.svg
[ci]: https://github.com/kubemod/kubemod/actions
[ci-img]: https://gitlab.com/kubemod/kubemod/badges/master/pipeline.svg
[goreport-img]: https://goreportcard.com/badge/github.com/kubemod/kubemod
[goreport]: https://goreportcard.com/report/github.com/kubemod/kubemod
[cov-img]: https://codecov.io/gh/kubemod/kubemod/branch/master/graph/badge.svg
[cov]: https://codecov.io/github/kubemod/kubemod/
