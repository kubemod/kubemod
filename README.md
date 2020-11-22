[![Build Status][ci-img]][ci] [![Go Report Card][goreport-img]][goreport] [![Code Coverage][cov-img]][cov]

# KubeMod

KubeMod is a universal [Kubernetes mutating operator](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/).

It introduces `ModRule` &mdash; a custom Kubernetes resource which allows you to intercept the deployment of any Kubernetes object and apply targeted modifications to it before it is deployed to the cluster.

Use KubeMod to:

* Customize opaque Helm charts and Kubernetes operators
* Build a system of policy rules to reject misbehaving resources
* Develop your own sidecar container injections - no coding required

---

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
