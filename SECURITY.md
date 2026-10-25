# Security policy

## Report a vulnerability

Use [GitHub's private vulnerability report form](https://github.com/JohanLindvall/diz/security/advisories/new) to report a suspected vulnerability. Please avoid publishing exploit details in a public issue before the report has been reviewed.

Include the diz commit or build, Go and Docker versions, affected command or endpoint, reproduction steps, and the impact. A small sanitized archive or a public image reference is especially helpful. Do not include registry credentials or private images.

If private reporting is unavailable, contact [Johan Lindvall](mailto:johan.lindvall@gmail.com) using the subject `diz security report`.

## Supported code

Security fixes target the current `master` branch. There is no published long-term support schedule or guaranteed response time. Update to the latest code when checking whether an issue is already fixed.

## Running diz

The embedded registry listens on all network interfaces at port 5000 over unauthenticated HTTP. Every image and downloadable archive entry it serves is accessible to clients that can reach that port. Use a trusted network or add an authenticated TLS proxy when broader access is required.

Use trusted image archives. The `.hashes` index contains content hashes for lookup; it does not authenticate the archive's publisher. See the [format and compatibility notes](docs/archive-format.md) for the current boundaries.
