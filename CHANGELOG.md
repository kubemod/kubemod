# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

- 87: Extend JSPONPath with select functions isUndefined, isDefined, isEmpty and isNotEmpty
- 70: Add support for ARM64

## 0.14.0 - 2022-01-29

- 84: Increase dragnet webhook timeout to 10 seconds
- 71: Implement TLS certificate rotation
- 80: Update kubemod-crt to v1.1.3 (enables deployment to OpenShift with restricted user id ranges)
- 72: Upgrade to wire 0.5.0 and golang 1.17
- 67: Update upgrade instructions to explicitly delete certificate generation job

## 0.13.0 - 2021-05-15

- 65: Target apiextensions.k8s.io/v1 for CRDs
- 63: Remove kubemod-system namespace normalization

## 0.12.0 - 2021-04-19

- 51: Introduce TargetNamespaceRegex as a way to target resources across namespaces (thank you @jamiecore)
- 56: Implement native unified diffs (thank you @jamiecore)
- 57: Enable github actions for pull requests

## 0.11.0 - 2021-04-16

- 48: Extend Go Template engine with Sprig Functions

## 0.10.0 - 2021-01-30

- 45: Implement stability improvements for multi-node clusters

## 0.9.1 - 2021-01-22

- 43: Limit the default set of target resources

## 0.9.0 - 2020-01-09

- 41: Introduce support for cluster-wide resources
- 36: Update last-applied-configuration annotation when patching a resource
- 18: Implement /v1/dryrun API

## 0.8.3 - 2020-12-22

- 33: Do not fail the whole patch when one ModRule fails
- 31: Enable patching non-existent arrays with -1

## 0.8.2 - 2020-12-21

- 29: Log missing JSONPath keys as DBG level messages

## 0.8.1 - 2020-12-18

- 27: Validation fails for ModRules without rejectMessage

## 0.8.0 - 2020-12-18

- 25: Introduce matchFor
- 23: Add message field to Reject rules
- 21: Implement Reject ModRules

## 0.7.1 - 2020-11-27

- 10: Implement a more forgiving add patch operation

## 0.7.0 - 2020-11-22

- 12: Add ability to construct patches based on select expression
- 8: Switch to nonroot certificate generation

## 0.6.0 - 2020-10-15

- 5: Document ModRule spec
- 3: Implement GitHub Actions CI

## 0.5.0 - 2020-10-07

- 1: Document KubeMod use cases

## 0.4.2 - 2020-10-07

Initial release
