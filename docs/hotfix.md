# Hotfix Workflow

## Hotfix Flow

1. Create a branch from the latest stable branch.
2. Reproduce the issue with the smallest possible test or command.
3. Apply the minimal fix.
4. Run smoke tests.
5. Update changelog or release notes when user-facing behavior changes.
6. Open a pull request and describe the rollback risk.

## Branch Naming

Use short names:

```text
hotfix/v0.1.1-connect-timeout
hotfix/v0.2.1-doh-fallback
```

## Patch Versioning

Use patch releases for:

- bug fixes;
- small compatibility fixes;
- logging or documentation corrections;
- CI adjustments;
- low-risk test coverage improvements.

Examples:

```text
v0.1.0 -> v0.1.1
v1.0.0 -> v1.0.1
```

## Release Notes

Release notes should include:

- user-visible impact;
- affected modes;
- fix summary;
- verification commands;
- rollback notes.

## Rollback Notes

For any hotfix touching system proxy, PAC, hosts, certificates, or restore behavior, document:

- what system state is modified;
- how rollback is recorded;
- how `steam-accelerator restore` should recover;
- what users can do manually if restore fails.
