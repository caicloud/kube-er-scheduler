# kube-er-scheduler

## About the project

Kubernetes extended resource scheduler.

Scheduler is responsible for filtering nodes for pods requesting extended resources instead of lifecycle predicate admin handler. So scheduler can watch both ExtendedResources and ExtendedResourceClaims, and select the right ExtendedResources for ExtendedResourceClaims and set their status (we may also create a new controller to do this).

## Design

[Extended Resource Management Proposal](https://docs.google.com/document/d/13sAwOFJpreysF-9_HTMGmFgGOGKFHMN8GrFIL_EMaTw/edit)

## Status

The project is in pending status.

## Getting started
