[![Build Status][ci-img]][ci] [![Go Report Card][goreport-img]][goreport] [![Code Coverage][cov-img]][cov]

# KubeMod

KubeMod is a universal [Kubernetes mutating operator](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/).

It introduces `ModRule` &mdash; a custom Kubernetes resource which allows you to intercept the deployment of any Kubernetes object and apply targeted modifications to it or reject it before it is deployed to the cluster.

Use KubeMod to:

* Customize opaque Helm charts and Kubernetes operators.
* Build a system of policy rules to reject misbehaving resources.
* Develop your own sidecar container injections - no coding required.

## Table of contents

* [Installation](#installation)
* [Deploying our first ModRule](#deploying-our-first-modrule)
* [Common use cases](#common-use-cases)
    * [Modification of behavior](#modification-of-behavior)
    * [Modification of metadata](#modification-of-metadata)
    * [Sidecar injection](#sidecar-injection)
    * [Resource rejection](#resource-rejection)
* [Understanding ModRules](#understanding-modrules)
    * [Match section](#match-section)
    * [Patch section](#patch-section)
* [Miscellaneous](#miscellaneous)
    * [Debugging ModRules](#debugging-modrules)
    * [Gotchas](#gotchas)

---

## Installation

As a Kubernetes operator, KubeMod is deployed into its own namespace — `kubemod-system`.  
The following command will create namespace `kubemod-system` and will deploy KubeMod into it.

```bash
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.8.1/bundle.yaml
```

### Upgrade

If you are upgrading from a previous version of KubeMod, run the following:

```bash
# Delete the kubemod certificate generation job in case KubeMod has already been installed.
kubectl delete job -l job-name -n kubemod-system

# Upgrade KubeMod operator.
kubectl apply -f https://raw.githubusercontent.com/kubemod/kubemod/v0.8.1/bundle.yaml
```

### Uninstall

To uninstall KubeMod and all its resources, run:

```text
kubectl delete -f https://raw.githubusercontent.com/kubemod/kubemod/v0.8.1/bundle.yaml
```

**Note**: Uninstalling KubeMod will also remove all your ModRules deployed to all Kubernetes namespaces.

## Deploying our first ModRule

Once KubeMod is installed, you can deploy ModRules to intercept the creation and update of specific resources and perform modifications on them.

For example, here's a ModRule which intercepts the creation of Deployment resources whose `app` labels equal `nginx` and include at least one container of `nginx` version `1.14.*`.

The ModRule patches the matching Deployments on-the-fly to enforce a specific `securityContext` and add annotation `my-annotation`.

Since KubeMod intercepts and patches resources **before** they are deployed to Kubernetes, we are able to patch read-only fields such as `securityContext` without the need to drop and recreate existing resources.

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

    # ... but has no explicit runAsNonRoot security context.
    # Note: "negate: true" flips the meaning of the match.
    - select: '$.spec.template.spec.securityContext.runAsNonRoot == true'
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

Save the above `ModRule` to file `my-modrule.yaml` and deploy it to the default namespace of your Kubernetes cluster:

```bash
kubectl apply -f my-modrule.yaml
```

After the `ModRule` is created, the creation of any nginx Kubernetes `Deployment` resource in the same namespace will be intercepted by KubeMod, and if the `Deployment` resource matches the ModRule's `match` section, the resource will be patched with the collection of `patch` operations.

To list all ModRules deployed to a namespace, run the following:

```bash
kubectl get modrules
```

## Common use cases

The development of KubeMod was motivated by the proliferation of Kubernetes Operators and Helm charts which are sometimes opaque to customizations and lead to runtime issues.

For example, consider these issues:

* [https://github.com/elastic/cloud-on-k8s/issues/2328](https://github.com/elastic/cloud-on-k8s/issues/2328)
* [https://github.com/jaegertracing/jaeger-operator/issues/1096](https://github.com/jaegertracing/jaeger-operator/issues/1096)

Oftentimes these issues are showstoppers that render the chart/operator impossible to use for certain use cases.

With the help of KubeMod we can make those charts and operators work for us. Just deploy a `ModRule` which targets the problematic primitive resource and patch it on the fly at the time it is created.

See the following sections for a number of typical use cases for KubeMod.


### Modification of behavior

Here's a typical black-box operator issue which can be fixed with KubeMod: [https://github.com/elastic/cloud-on-k8s/issues/2328](https://github.com/elastic/cloud-on-k8s/issues/2328).

The issue is that when the [Elastic Search operator](https://github.com/elastic/cloud-on-k8s) creates Persistent Volume Claims, it attaches an `ownerReference` to them such that they are garbage-collected after the operator removes the Elastic Search stack of resources.

This makes sense when we plan to dynamically scale Elastic Search up and down, but it doesn't make sense if we don't plan to scale dynamically, but we do want to keep the Elastic Search indexes during Elastic Search reinstallation \(see comments [here](https://github.com/elastic/cloud-on-k8s/issues/2328#issuecomment-583254122) and [here](https://github.com/elastic/cloud-on-k8s/issues/2328#issuecomment-650335893)\).

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

### Modification of metadata

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

With the help of ModRules, one can dynamically inject arbitrary sidecar containers into `Deployment` and `StatefulSet` resources. The `patch` part of the ModRule is a [Golang template](https://golang.org/pkg/text/template/) which takes the target resource object as an intrinsic context allowing for powerful declarative rules such as the following one which injects a [Jaeger Agent](https://www.jaegertracing.io/docs/1.19/architecture/#agent) sidecar into any Deployment tagged with annotation `my-inject-annotation` set to `"true"`:

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

Note the use of `{{ .Target.metadata.name }}` in the patch `value` to dynamically access the name of the deployment being patched and pass it to the Jaeger agent as a tracer tag.

When a patch is evaluated, KubeMod executes the patch value as a [Golang template](https://golang.org/pkg/text/template/) and passes the following intrinsic items accessible through the template's context:

* `.Target` - the original resource object being patched with all its properties.
* `.Namespace` - the namespace of the resource object.


### Resource rejection

There are two types of ModRules — `Patch` and `Reject`.

All of the examples we've seen so far have been of type `Patch`.

`Reject` ModRules are simpler as they only have a `match` section.

If a resource matches the `match` section of a `Reject` ModRule, its creation/update will be rejected. This enables the development of a system of policy ModRules which enforce certain security restrictions in the namespace they are deployed to.

For example, here's a `Reject` ModRule which prevents the infamous [CVE-2020-8554: Man in the middle using ExternalIPs](https://github.com/kubernetes/kubernetes/issues/97076):

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: reject-malicious-external-ips
spec:
  type: Reject

  rejectMessage: 'One or more of the following external IPs are not allowed {{ .Target.spec.externalIPs }}'

  match:
    # Reject Service resources...
    - select: '$.kind'
      matchValue: 'Service'

    # ...with non-empty externalIPs...
    - select: 'length($.spec.externalIPs) > 0'

    # ...where some of the IPs were not part of the allowed subnet 123.45.67.0/24.
    - select: '$.spec.externalIPs[*]'
      matchFor: All
      matchRegex: '123\.45\.67\.*'
      negate: true
```


Here's another ModRule which rejects the deployment of any `Deployment` or `StatefulSet` resource that does not explicitly require non-root containers:

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule
metadata:
  name: reject-root-access-workloads
spec:
  type: Reject

  rejectMessage: 'All workloads must run as non-root user'
  
  match:
    # Reject Deployments and StatefulSets...
    - select: '$.kind'
      matchValues:
        - 'Deployment'
        - 'StatefulSet'

    # ...that have no explicit runAsNonRoot security context.
    - select: "$.spec.template.spec.securityContext.runAsNonRoot == true"
      negate: true
```

---

## Understanding ModRules

A `ModRule` has a `type`, a `match` section, and a `patch` section.

```yaml
apiVersion: api.kubemod.io/v1beta1
kind: ModRule

spec:
  type: ...

  match:
    ...

  patch:
    ...
```

The `type` of a `ModRule` can be one of the following:

* `Patch` — this type of `ModRule` applies patches to objects that match the `match` section of the rule. Section `patch` is required for `Patch` ModRules.
* `Reject` — this type of `ModRule` rejects objects which match the `match` section. When `type` is `Reject`, the spec accepts an optional `rejectMessage` field.

Section [`match`](#match-section) is an array of individual criteria items used to determine if the `ModRule` applies to a Kubernetes object.

Section [`patch`](#patch-section) is an array of patch operations.

### Match section

Section `match` of a `ModRule` is an array of individual criteria items.

When a new object is deployed to Kubernetes, or an existing one is updated, KubeMod intercepts the operation and attempts to match the object's definition against all `ModRules` deployed to the namespace where the object is being deployed.

A `ModRule` is considered to have a match with the Kubernetes object definition when all criteria items in its `match` section yield a positive match.

#### Criteria item

A criteria item contains a required `select` expression and optional `matchValue`, `matchValues`, `matchRegex` and `negate` fields.

For example, the following `match` section has two criteria items. This `ModRule` will match all resources whose `kind` is equal to `Deployment` **and** have a container name that's either `container-1` or `container-2` .

```yaml
...
  match:
    - select: '$.kind'
      matchValue: 'Deployment'

    - select: '$.spec.template.spec.containers[*].name'
      matchValues:
        - 'container-1'
        - 'container-2'
```

A criteria item is considered a positive match when:

* its `select` expression yields a single boolean `true` value.
* its `select` expression yields one or more non-boolean values **and** one of the following is true:
  * Fields `matchValue`, `matchValues` and `matchRegex` are not specified.
  * `matchValue` is specified and:
    * `matchFor` is set to `Any` (or unspecified) and one or more of the values resulting from `select` exactly matches the value of `matchValue`.
    * `matchFor` is set to `All` and all of the values resulting from `select` exactly match the value of `matchValue`.
  * `matchValues` is specified and:
    * `matchFor` is set to `Any` (or unspecified) and one or more of the values resulting from `select` exactly matches one of the values in `matchValues`.
    * `matchFor` is set to `All` and all of the values resulting from `select` exactly match one of the values in `matchValues`.
  * `matchRegex` is specified and:
    * `matchFor` is set to `Any` (or unspecified) and one or more of the values resulting from `select` matches that regular expression.
    * `matchFor` is set to `All` and all of the values resulting from `select` match that regular expression.

The result of a criteria item can be inverted by setting its `negate` field to `true`.

A criteria item whose `select` expression yields no results is considered non-matching unless it is `negated`.

##### `select` \(string : required\)

The `select` field of a criteria item is a [JSONPath](https://goessner.net/articles/JsonPath/) expression.

When a `select` expression is evaluated against a Kubernetes object definition, it yields zero or more values.

Let's consider the following JSONPath select expression:

```javascript
$.spec.template.spec.containers[*].name
```

When this expression is evaluated against a `Deployment` resource definition whose specification includes three containers, the result of this `select` expression will be a list of the names of those three containers.

If `select` yields a single boolean value, that value is considered to be the result of the match regardless of the values of `matchValue`, `matchValues` and `matchRegex`.

For any other case, KubeMod converts the result of the `select` expression to a list of strings, regardless of what the original type of the target field is.

Here's another example:

```javascript
$.spec.template.spec.containers[*].ports[*].containerPort
```

This expression will yield a list of all `containerPort` values for all ports and all containers. The values in the list will be the string representation of those port numbers.

###### `select` filters

KubeMod `select` expressions includes an extension to JSONPath — filters.

A filter is an expression in the form of `[? <filter expression>]` and can be used in place of JSONPath's `[*]` to filter the elements of an array.

Let's take a look at the following `select` expression:

```javascript
$.spec.template.spec.containers[*].ports[? @.containerPort == 8080]
```

The expression `[? @.containerPort == 8080]` filters the result of the `select` to include only containers that include a port with whose `containerPort` field equals `8080`.

The filter expression could be any JavaScript boolean expression.

The special character `@` represents the current object the filter is iterating over. In the above filter expression, that is the current element of the `ports` array.

##### `matchFor` \(string: optional\)

Field `matchFor` controls how `select` results are evaluated against `matchValue`, `matchValues` and `matchRegex`.

The value of `matchFor` can be either `Any` or `All`. When not specified, `matchFor` defaults to `Any`.

See below for more information on how `matchFor` impacts the results of a match.


##### `matchValue` \(string: optional\)

When present, the value of field `matchValue` is matched against the results of `select`.

If `matchFor` is set to `Any` and any of the items returned by `select` match `matchValue`, the match criteria is considered a positive match.

If `matchFor` is set to `All` and all of the items returned by `select` match `matchValue`, the match criteria is considered a positive match.

The match performed by `matchValue` is case sensitive. If you need case insensitive matches, use `matchRegex`.

##### `matchValues` \(array of strings: optional\)

Field `matchValues` is an array of strings which are tested against the results of `select`.

If `matchFor` is set to `Any` and any of the items returned by `select` match any of the `matchValues`, the match criteria is considered a positive match.

If `matchFor` is set to `All` and all of the items returned by `select` match any of the `matchValues`, the match criteria is considered a positive match.

This match is case sensitive. If you need case insensitive matches, use `matchRegex`.

##### `matchRegex` \(string: optional\)

Field `matchRegex` is a regular expression matched against the results of `select`.

If `matchFor` is set to `Any` and any of the items returned by `select` match `matchRegex`, the match criteria is considered a positive match.

If `matchFor` is set to `All` and all of the items returned by `select` match `matchRegex`, the match criteria is considered a positive match.

##### `negate` \(boolean: optional\)

Field `negate` can be used to flip the outcome of the criteria item match. Its default value is `false`.

### Patch section

Section `patch` is an array of [RFC6902 JSON Patch](https://tools.ietf.org/html/rfc6902) operations.

KubeMod's variant of JSON Patch includes the following extensions to RFC6902:

* Negative array indices mean starting at the end of the array.
* Operations which attempt to remove a non-existent path in the JSON object are ignored.

#### Patch operation

A patch operation contains fields `op`, `select`, `path` and `value`.

For example, the following `patch` section applies two patch operations executed against every `Deployment` object deployed to the namespace where the `ModRule` resides.

```yaml
...
  match:
    - select: '$.kind'
      matchValue: 'Deployment'

  patch:
    - op: add
      path: /metadata/labels/color
      value: whatever

    # Change all nginx containers' ports from 80 to 8080
    - op: add
      select: '$.spec.template.spec.containers[? @.image =~ "nginx" ].ports[? @.containerPort == 80]'
      path: '/spec/template/spec/containers/#0/ports/#1/containerPort'
      value: '8080'
```

The first patch operation adds a new `color` label to the target object.

The second one switches the `containerPort` of all `nginx` containers present in the target `Deployment` object from `80` to `8080`.

Following is a break-down of each field of a patch operation.

##### `op` \(string: required\)

Field `op` indicates the type of patch operation to be performed against the target object. It can be one of the following:

* `replace` — this type of operation replaces the value of element represented by `path` with the value of field `value`. If `path` points to a non-existent element, the operation fails.
* `add` — this type of operation adds the element represented by `path` with the value of field `value`. If the element already exists, `add` behaves like `replace`.
* `remove` — this type of operation removes the element represented by `path`. If `path` points to a non-existent element, the operation is ignored.

##### `select` \(string: optional\) and `path` \(string: required\)

The `select` field of a patch item is a [JSONPath](https://goessner.net/articles/JsonPath/) expression.

When a `select` expression is evaluated against a Kubernetes object definition, it yields zero or more values.

For more information about `select` expressions, see [Match item select expressions](#select-string--required).

When `select` is used in a patch operation, the patch is executed once for each item yielded by `select`.

If the `select` item uses JSONPatch wildcards \(such as `..` or `[*]`\) and/or [select filters](#select-filters), KubeMod captures the zero-based index of each wildcard/filter result and makes it available for use in the `path` expression.

Let's consider the following example:

```yaml
op: add
select: '$.spec.template.spec.containers[*].ports[? @.containerPort == 80]'
path: '/spec/template/spec/containers/#0/ports/#1/containerPort'
value: '8080'
```

The `select` expression includes a wildcard to loop over all containers \(`containers[*]`\), and then a filter \(`ports[? @.containerPort == 80]`\) to select only the ports whose `containerPort` is equal to `80`.

If we evaluate this `select` expression against a `Deployment` with the following four containers and port objects...

```yaml
...
  containers:
    - name: c1
      ports:
        - containerPort: 100
          name: abc
        - containerPort: 200
          name: xyz

    - name: c2
      ports:
        - containerPort: 100
          name: abc
        - containerPort: 80
          name: xyz

    - name: c3
      ports:
        - containerPort: 100
          name: abc
        - containerPort: 200
          name: xyz

    - name: c4
      ports:
        - containerPort: 80
          name: abc
        - containerPort: 200
          name: xyz
        - containerPort: 300
          name: foo
...
```

... the `select` expression will yield the following two items:

* Item 1: The second port object of the second container
* Item 2: The first port object of the fourth container

The zero-based wildcard/filter indexes captured by KubeMod for this `select` will be as follows:

* Item 1:
  * container index: 1
  * port index: 1
* Item 2:
  * container index: 3
  * port index: 0

The indexes above can be used when constructing the `path` of the patch operation.

For example:

```yaml
path: '/spec/template/spec/containers/#0/ports/#1/containerPort'
```

The index placeholders `#0` and `#1` refers to the first and the second index captured by KubeMod when evaluating the `select` expression.

In our previous `select` example, `#0` would be the placeholder for the container index, and `#1` would be the placeholder for the port index.

When KubeMod performs patch operations, it constructs the `path` by replacing the index placeholders with the value of the corresponding indexes.

For the above `select` and `path` examples executed against our sample `Deployment`, KubeMod will generate two patch operations which will target the following paths:

* `/spec/template/spec/containers/1/ports/1/containerPort`
* `/spec/template/spec/containers/3/ports/0/containerPort`

Combining `select` expressions with `path`s with index placeholders gives us the ability to perform sophisticated targeted resource modifications.

If `select` is not specified, `path` is rendered as-is and is not subject to index placeholder interpolation.

##### value \(string\)

`value` is required for `add` and `replace` operations.

`value` is the **string representation** of a `YAML` value. It can represent a primitive value or a complex `YAML` object or array.

Here are a few examples:

**Number** \(note the quotes - `value` itself is a string, but its "value" evaluates to a `YAML` number\):

```yaml
value: '8080'
```

**String**:

```yaml
value: hello
# or
value: 'hello'
```

String representation of a number \(note the double-quotes\):

```yaml
value: '"8080"'
```

**Boolean**:

```yaml
value: 'false'
```

String representation of a boolean:

```yaml
value: '"false"'
```

**YAML object** \(note `|-` which makes `value` a multi-line string\):

```yaml
value: |-
  name: jaeger-agent
  image: jaegertracing/jaeger-agent:1.18.1
  imagePullPolicy: IfNotPresent
  ports:
  - containerPort: 6832
    name: jg-binary-trft
    protocol: UDP
```

###### Golang Template

When `value` contains `{{ ... }}`, it is evaluated as a [Golang template](https://golang.org/pkg/text/template/).

The following intrinsic item are accessible through the template's context:

* `.Target` — the original resource object being patched.
* `.Namespace` — the namespace of the target object.

For example, the following excerpt of a Jaeger side-car injection `ModRule` includes a `value` which uses `{{ .Target.metadata.name }}` to access the name of the `Deployment` being patched.

```yaml
...
value: |-
  name: jaeger-agent
  image: jaegertracing/jaeger-agent:1.18.1
  imagePullPolicy: IfNotPresent
  args:
    - --jaeger.tags=deployment.name={{ .Target.metadata.name }}
  ports:
  - containerPort: 6832
    name: jg-binary-trft
    protocol: UDP
...
```

See full example of the above ModRule [here](#sidecar-injection).

### rejectMessage

Field `rejectMessage` is an optional message displayed when a resource is rejected by a `Reject` ModRule.
The field is a Golang template evaluated in the context of the object being rejected

## Miscellaneous

### Debugging ModRules

To list the ModRules deployed to a namespace, run the following command:

```bash
kubectl get modrules
```

When a ModRule does not behave as expected, your best bet is to analyze KubeMod's operator logs.

Follow these steps:

* Deploy your ModRule to the namespace where you will be deploying the Kubernetes resources the ModRule should intercept.
* Find the KubeMod operator pod - run the following command and grab the name of the pod that begins with `kubemod-operator`:

```bash
kubectl get pods -n kubemod-system
```

* Tail the logs of the pod:

```bash
kubectl logs kubemod-operator-xxxxxxxx-xxxx -n kubemod-system -f
```

* In another terminal deploy the Kubernetes object your ModRule is designed to intercept.
* Watch the operator pod logs.

If your ModRule is a Patch rule, KubeMod operator will log the full JSON Patch applied to the target Kubernetes object at the time of interception.

If there are any errors at the time the patch is calculated, you will see them in the logs.

If the operator log is silent at the time you deploy the target object, this means that your ModRule's `match` criteria did not yield a positive match for the target object.


### Gotchas

When multiple ModRules match the same resource object, all of the ModRule patches are executed against the object in an indeterminate order.

This is by design.

Be careful when creating ModRules such that their match criteria and patch sections don't overlap leading to unexpected behavior.


[ci-img]: https://github.com/kubemod/kubemod/workflows/Master%20Workflow/badge.svg
[ci]: https://github.com/kubemod/kubemod/actions
[ci-img]: https://gitlab.com/kubemod/kubemod/badges/master/pipeline.svg
[goreport-img]: https://goreportcard.com/badge/github.com/kubemod/kubemod
[goreport]: https://goreportcard.com/report/github.com/kubemod/kubemod
[cov-img]: https://codecov.io/gh/kubemod/kubemod/branch/master/graph/badge.svg
[cov]: https://codecov.io/github/kubemod/kubemod/
