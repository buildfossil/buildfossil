# Docker Snapshot Resource Inspection

REW provides a read-only diagnostic command:

    rew cleanup --dry-run

The command lists Docker snapshot volume candidates whose names and
metadata match REW's schema-v3 snapshot conventions.

## What it does

- Queries Docker Engine for volumes.
- Validates snapshot volume names and expected labels.
- Displays matching volume names and kinds.
- Reports Docker API errors and listing warnings.
- Makes no Docker resource changes.

## Limitations

This command currently inspects snapshot volumes only. It does not
inspect preparation containers or execution containers.

A matching name and set of Docker labels does not establish the
resource's authentic origin or prove that it is orphaned.

The command does not determine whether a volume is in use and does
not remove any resources.

An empty result means no matching snapshot volume candidates were
found at inspection time. It does not guarantee that all REW resources
have been removed.

## Security

Docker labels are metadata, not authentication.

Do not automatically delete matching resources or use a broad
docker volume prune command as a substitute for ownership checks.

For REW's threat model, see security-model.md.
