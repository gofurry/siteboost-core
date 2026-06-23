# web-boost Library Extraction Plan

Canonical Chinese plan: [zh/web-boost-library-plan.md](zh/web-boost-library-plan.md).

Target repository: [gofurry/web-boost](https://github.com/gofurry/web-boost).

`web-boost` is the future reusable Go library. It should extract only validated core capabilities from this experimental repository and must not inherit the current Steam naming, CLI shape, AppHost service installer, local smoke scaffolding, or experimental repository layout.

## Required Boundaries

- Root package `webboost` exposes only the small stable API: `Config`, `Engine`, `Provider`, `Mode`, `Status`, `Start`, `Stop`, and `Restore`.
- Capabilities are grouped under clear packages such as `provider`, `rules`, `network/resolver`, `network/upstream`, `takeover/*`, `reverse`, `pageenhance`, `certstore`, `rollback`, `diagnostics`, and `adapters/*`.
- Do not scatter every capability in the repository root.
- System-changing behavior must be explicit, observable, rollback-backed, and restorable.
- Provider packages must not own hosts, certificate, DNS takeover, AppHost, or system modification responsibilities.
- AppHost service installation and desktop product UX stay out of the core library.
- TUN/VPN stays deferred to external libraries or separate integrations.

See the Chinese plan for the detailed directory tree, API sketch, package boundaries, and v0.8.0 deliverables.
