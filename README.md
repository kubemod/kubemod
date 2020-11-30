[![Build Status][ci-img]][ci] [![Go Report Card][goreport-img]][goreport] [![Code Coverage][cov-img]][cov]

# KubeMod

KubeMod is a universal [Kubernetes mutating operator](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/).

It introduces `ModRule` &mdash; a custom Kubernetes resource which allows you to intercept the deployment of any Kubernetes object and apply targeted modifications to it before it is deployed to the cluster.

Use KubeMod to:

* Customize opaque Helm charts and Kubernetes operators.
* Build a system of policy rules to reject misbehaving resources.
* Develop your own sidecar container injections - no coding required.

---

## Example

Here's a ModRule which intercepts the creation of Deployment resources whose `app` labels equal `nginx` and include at least one container of `nginx` version `1.14.*`.

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
      value: hello

    # Enforce non-root securityContext and make nginx run as user 101.
    - op: add
      path: /spec/template/spec/securityContext
      value: |-
        fsGroup: 101
        runAsGroup: 101
        runAsUser: 101
        runAsNonRoot: true
```

## Documentation

To find out how to install and use KubeMod, head on over to the docs:

:point_right: [docs.kubemod.io](https://docs.kubemod.io)

[ci-img]: https://github.com/kubemod/kubemod/workflows/Master%20Workflow/badge.svg
[ci]: https://github.com/kubemod/kubemod/actions
[ci-img]: https://gitlab.com/kubemod/kubemod/badges/master/pipeline.svg
[goreport-img]: https://goreportcard.com/badge/github.com/kubemod/kubemod
[goreport]: https://goreportcard.com/report/github.com/kubemod/kubemod
[cov-img]: https://codecov.io/gh/kubemod/kubemod/branch/master/graph/badge.svg
[cov]: https://codecov.io/github/kubemod/kubemod/
