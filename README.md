
# BlanketOps Environments — Conformance Tests

Test suite for the BlanketOps Environments API surface. Exercises every custom
resource across all four domains — `environments`, `events`, `networks`,
`sources` — against shared fixtures, fake controllers, and the gRPC contract.

This repository contains  **no production code** . It exists to answer one
question continuously: *does an implementation of the Environments API behave
the way the contract says it must?*

## Relationship to other repositories

| Repository                             | Role                                                      |
| -------------------------------------- | --------------------------------------------------------- |
| `blanketops-environments-contract`   | Proto definitions — the contract under test              |
| `blanketops-environments-controller` | Reference implementation the fakes mirror                 |
| `blanketops-environments-tests`      | **This repo**— fixtures, fakes, conformance suites |

The pipeline chain the resources participate in:

```
GitRepository → GitHubEvent → Build → SupplyChain → Package → Deployment
```

## Layout

```
tests/
├── data/      Canonical test fixtures — one file per CR, one package per domain
├── fake/      Fake domain controllers + behavioural suites (Ginkgo/Gomega)
└── grpc/      gRPC service conformance tests against the contract
```

### `tests/data` — fixtures

Constructor functions producing valid, canonical instances of every CR.
These are the single source of truth for "what a well-formed resource looks
like" — fakes and gRPC suites both consume them. Every fixture carries the
required BlanketOps labels:

```
environments.blanketops.dev/name
environments.blanketops.dev/type
environments.blanketops.dev/api-version
environments.blanketops.dev/contract-version
```

When the contract changes shape, fixtures change first; failures ripple
outward from here by design.

### `tests/fake` — behavioural suites

Deterministic fake controllers per domain, exercised by `suite_test.go` files.
These encode the *behavioural* contract — phase transitions, status writes,
ownership semantics — without requiring a cluster. A fake that drifts from the
reference controller is a bug in one of the two; the suite is how you find out
which.

### `tests/grpc` — service conformance

One test file per service (`BuildService`, `DeploymentService`,
`GitRepositoryService`, …) validating the generated gRPC surface: request and
response shapes, CRUD + domain-specific RPCs, watch-stream semantics. Tests
not yet implemented are stubbed with `t.Skip()` — a skip is a TODO, not a
pass.

## Running

```bash
# everything
make test

# one layer
go test ./tests/fake/...
go test ./tests/grpc/...

# one domain
go test ./tests/fake/environments/...

# verbose Ginkgo output
go test ./tests/fake/... -v
```

Requires Go (version per `go.mod`). No cluster, no external services — the
suite is fully self-contained and deterministic.

## CI

Two workflows run on `ubuntu-latest` GitHub-hosted runners:

* **`run-tests.yml`** — full suite on push/PR. Emits JUnit XML via
  `go-junit-report`, surfaced as a check through `dorny/test-reporter`.
* **`create-test.yml`** — on every PR merged into `main`, files a tracking
  issue for each test file that PR touched (not a full-tree rescan). Also
  runnable via `workflow_dispatch` for a one-off full scan.

## Conventions

* **Fixtures are canonical.** Never hand-roll a resource inside a test;
  consume `tests/data` and mutate only the field under test.
* **One domain, one package.** A new CR gets a fixture file, a fake, a suite
  registration, and a gRPC test file — all four, in the same change.
* **Skips are visible debt.** `t.Skip()` marks unimplemented conformance, and
  CI reports skips distinctly from passes.
* **Determinism is non-negotiable.** No sleeps, no wall-clock assertions, no
  network. If a test needs ordering, it asserts on observed state.

## Adding a domain

1. Fixture constructors in `tests/data/<domain>/`
2. Fake controller + behaviours in `tests/fake/<domain>/` with a
   `suite_test.go`
3. gRPC conformance in `tests/grpc/<resource>_grpc_test.go`
4. Wire into `make test` (automatic via `./tests/...` globbing)

## License

See [LICENSE](https://claude.ai/chat/LICENSE).
