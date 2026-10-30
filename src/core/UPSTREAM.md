# OpenFlux upstream

- Release: [v0.17.1](https://github.com/p1neappleXpress/OpenFlux/releases/tag/v0.17.1).
- Commit: `f492a1a33b7581ad8f88118f97e0b866adda3cdc`.
- Source archive: `https://github.com/p1neappleXpress/OpenFlux/archive/refs/tags/v0.17.1.zip`.
- Archive SHA-256: `249bb2269e3d70d1f1fa94f25431337471ef65026dd16904288dd2d3ba2559cd`.

The complete server-side upstream delta from v0.13.0 is included. The previous
Windows/Linux scope and all local patches were audited before the three-way migration.
See the integration notes in the migration docs.

Preserved patches: Telemost per-type URL; transport-scoped cookie jars with
legacy fallback; logical IPv4 metering and optional IPC counters/status requests;
peer activity and Session expiry; Linux SO_MARK for the panel's scoped L3 guard;
Windows/UDP/heartbeat/PHP and Mail.ru batching regression tests. Mobile and iOS
applications and their build scripts remain excluded from this server tree.

New Server integration: Bitrix24 and mixed URL flags, --transport-options-file for
private upstream registry parameters, and Bitrix24 on the classic CLI path.
The share format retains the official transport allowlist. Mixed groups
and Bitrix24 credentials are omitted from links with an explanation; both need
manual client configuration. Wire framing/encryption stay upstream.

Avito room-creation diagnostics additionally report progress and the first HTTP
or network failure at the normal log level. Exhausted startup attempts preserve
the underlying error instead of only returning "could not create any room".
The retry policy and room/wire formats remain unchanged.
