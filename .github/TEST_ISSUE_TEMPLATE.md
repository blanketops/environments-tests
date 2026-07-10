<!--
  Pick ONE section below and delete the others.
  Title prefix matches the section: [gap] / [fixture] / [fake] / [grpc] / [flaky]
  e.g. [gap] events: WatchGitHubEvent stream semantics unimplemented
-->

# 🧪 Conformance gap

<!-- A t.Skip() that needs to become a real test. Skips are visible debt. -->

## Skipped test

* Layer: `<!-- fake / grpc -->`
* Domain: `<!-- environments / events / networks / sources -->`
* Resource:
* Test file / function:

## What the contract requires

<!-- The behaviour or RPC surface the test must validate. Link the proto or
     ESP-0001 section if applicable. -->

## Blocked on

* [ ] Nothing — just unwritten
* [ ] Contract undecided: `<!-- link issue in the contract repo -->`
* [ ] Reference implementation incomplete: `<!-- link issue in the controller repo -->`

---

# 🧬 Fixture drift

<!-- tests/data no longer matches the contract, or a fixture violates canon. -->

## Fixture

* File: `<!-- tests/data/<domain>/<resource>.go -->`
* Constructor:

## Drift

<!-- What's wrong: missing required field, stale label set, shape diverged
     from proto, invalid against current CRD. -->

## Contract reference

<!-- Which contract change caused it — tag, commit, or proto file. -->

## Blast radius

* [ ] Single fixture
* [ ] Ripples into fake suites
* [ ] Ripples into gRPC suites
* [ ] Required labels affected (`environments.blanketops.dev/*`)

---

# 🎭 Fake vs reference mismatch

<!-- The fake controller and the reference controller disagree. One is wrong. -->

## Behaviour under test

* Domain / resource:
* Suite / spec: `<!-- file and test name -->`

## Fake behaviour

<!-- What the fake does — phase written, condition set, transition taken. -->

## Reference behaviour

<!-- What blanketops-environments-controller actually does. -->

## Verdict (if known)

* [ ] Fake is wrong — fix here
* [ ] Reference is wrong — issue filed in controller repo: `<!-- link -->`
* [ ] Contract is ambiguous — issue filed in contract repo: `<!-- link -->`

---

# 📡 gRPC contract failure

<!-- Generated service surface fails conformance. -->

## Service / RPC

* Service: `<!-- e.g. BuildService -->`
* RPC: `<!-- e.g. WatchBuild -->`
* Test file:

## Failure

```text
# exact test output
```

## Contract version

* Contract tag / commit:
* Generated code in sync? `<!-- buf generate run against that tag? -->`

## Classification

* [ ] Test wrong — bad assertion
* [ ] Generated code wrong — codegen / buf config issue
* [ ] Contract wrong — proto needs amendment: `<!-- link contract issue -->`

---

# 🌀 Flaky / non-determinism

<!-- Determinism is non-negotiable. A flake is a bug, never a retry. -->

## Test

* Layer / domain / file:
* Failure rate: `<!-- e.g. ~1 in 20 runs -->`

## Evidence

```text
# failing output, plus a passing run for contrast if useful
```

## Suspected source

* [ ] Ordering assumption (map iteration, unsorted slice, event order)
* [ ] Time (wall clock, sleep, timeout too tight)
* [ ] Shared state across specs (suite-level fixture mutation)
* [ ] Goroutine leak from a previous spec
* [ ] Unknown — needs bisection

## Reproduce

```bash
go test ./tests/... -run <TestName> -count=100
```
