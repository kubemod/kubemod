# Contributing to KubeMod

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
