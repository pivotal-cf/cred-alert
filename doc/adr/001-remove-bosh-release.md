# 1. Record architecture decisions

Date: 2018-07-02

## Status

Accepted

## Context

We no longer support global scanning of Github commits.

## Decision

Remove all bosh-release components, move `cred-alert-cli` code to the top-leve
of the repo.

## Consequences

Simplify maintenance load for team, `revok` (scanning server) is no longer
present.
