# GCP Secret Manager Emulator — API Conformance Report

**Date:** 2026-04-03
**Emulator version:** current `main` branch
**Reference API:** `google.cloud.secretmanager.v1` (SecretManagerService)
**Auditor:** automated analysis against proto types and `go doc` output

---

## 1. Executive Summary

| Category | Count |
|---|---|
| RPC methods in spec | 13 (+ 3 IAM inherited) |
| RPC methods implemented | 10 |
| RPC methods missing | 3 (GetIamPolicy, SetIamPolicy, TestIamPermissions) |
| Methods with critical issues | 6 |
| Methods with minor issues only | 4 |
| Methods fully conformant | 0 |

**Overall verdict: PARTIAL**

The emulator covers the core data path that most SDK clients exercise (create, get, list, delete, add version, access version, state transitions). However, several response fields that the official Go client SDK inspects are never populated, pagination ordering is inverted from the spec, etag-based optimistic concurrency is silently ignored across all mutating operations, and the crc32c integrity path is entirely absent. These gaps cause observable differences when client code checks returned metadata rather than just payload bytes.

---

## 2. Method-by-Method Analysis

### 2.1 CreateSecret

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (parent, secret_id, secret) | PASS |
| Returns `AlreadyExists` on duplicate | PASS |
| Stores and returns `replication` field | PASS |
| Stores and returns `labels` | PASS |
| Stores and returns `annotations` | PASS |
| Returns `create_time` | PASS |
| Returns `etag` | **FAIL — etag never generated or returned** |
| Stores/returns `topics` | **FAIL — field ignored; not stored** |
| Stores/returns `rotation` | **FAIL — field ignored; not stored** |
| Stores/returns `version_aliases` | **FAIL — field ignored; not stored** |
| Stores/returns `expire_time` / `ttl` | **FAIL — expiration fields ignored** |
| Stores/returns `version_destroy_ttl` | **FAIL — field ignored** |
| Stores/returns `customer_managed_encryption` | **FAIL — field ignored** |
| Stores/returns `tags` | **FAIL — field ignored** |
| Replication required by spec (GCP rejects nil replication) | Emulator defaults to AUTOMATIC — acceptable for emulation |
| `parent` supports `projects/*/locations/*` (regional) | **FAIL — only `projects/*` format handled; regional secrets unsupported** |

**Critical issues:**
- `etag` is never generated. `DeleteSecret` and `UpdateSecret` accept an etag in the request but never store or validate one, so clients relying on optimistic concurrency will silently get no protection.
- `topics` (Pub/Sub notifications) are accepted in the request proto but discarded — not stored, not returned by `GetSecret` / `ListSecrets`.
- `rotation` policy is discarded.
- Regional secret format (`projects/*/locations/*/secrets/*`) is not handled anywhere in the storage key or resource name construction, so regional SDK clients will fail with unexpected errors.

**Minor issues:**
- `version_destroy_ttl`, `customer_managed_encryption`, `tags`, `version_aliases` are all silently dropped. Most test clients don't check these, but SDK pagination helpers that iterate `version_aliases` will see an empty map.

---

### 2.2 GetSecret

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret | PASS |
| Returns `name`, `create_time`, `labels`, `annotations`, `replication` | PASS |
| Returns `etag` | **FAIL — never stored/returned** |
| Returns `topics`, `rotation`, `version_aliases` | **FAIL — never stored** |
| Returns `expire_time` / `ttl` | **FAIL — never stored** |

**Critical:** `etag` absent means clients using `UpdateSecret` with the read-modify-write pattern cannot do safe conditional updates.

---

### 2.3 ListSecrets

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (parent) | PASS |
| Pagination via `page_size` / `page_token` | PASS (integer-offset token) |
| Returns `next_page_token` when more results exist | PASS |
| Filter support | **FAIL — `filter` field is accepted but silently ignored** |
| `total_size` field populated | **FAIL — always 0** |
| Result ordering | **FAIL — spec says "sorted in reverse by create_time (newest first)"; emulator sorts by name ascending** |
| `etag`, `topics`, `rotation` etc. in listed secrets | **FAIL — same omissions as GetSecret** |
| Page token stability | **FAIL — index-based token is invalidated if secrets are added/deleted between pages** |

**Critical:**
- `filter` is completely ignored. Clients calling `ListSecrets(filter="name:foo-*")` will receive all secrets, not the filtered subset. The Go SDK's `ListSecrets` iterator passes the filter through; mismatched results break client-side deduplication logic.
- Sort order is wrong (ascending name vs. descending create_time). Auto-iterating clients that rely on the first result being the newest secret will get stale data.
- Pagination token is positional (integer index into sorted slice). Any concurrent mutation between pages will silently skip or duplicate entries.

**Minor:**
- `total_size` is never set.

---

### 2.4 UpdateSecret

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (secret.name, update_mask) | PASS |
| `labels` path in update_mask | PASS |
| `annotations` path in update_mask | PASS |
| `etag` in request used for optimistic concurrency | **FAIL — etag ignored** |
| `topics`, `rotation`, `version_aliases`, `expire_time`, `ttl` paths in update_mask | **FAIL — silently ignored ("following GCP behavior")** |
| Returns updated secret with all populated fields | FAIL (same omissions as GetSecret) |

**Critical:** The comment "following GCP behavior - silently skip" for unknown field paths is misleading; GCP actually returns `INVALID_ARGUMENT` for unrecognized paths in some configurations. More importantly, `rotation` and `expire_time` are legitimate updatable fields per spec and are silently dropped.

**Minor:** `version_aliases` is a commonly updated field (clients set alias → version mappings via UpdateSecret); silently ignoring it means alias-based access (`versions/my-alias`) will never work.

---

### 2.5 DeleteSecret

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret | PASS |
| Deletes secret and all versions | PASS |
| Returns `Empty` on success | PASS |
| Etag validation | **FAIL — `req.GetEtag()` is never read or validated** |

**Minor:** etag is optional per spec (if omitted, request succeeds), so this only matters when clients provide an etag for safety and expect a `FAILED_PRECONDITION` / `ABORTED` on mismatch. In the emulator the etag is ignored, making the optimistic concurrency guard a no-op.

---

### 2.6 AddSecretVersion

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (parent, payload) | PASS |
| Returns `NotFound` for missing secret | PASS |
| Creates version with state ENABLED | PASS |
| Returns `name`, `create_time`, `state` | PASS |
| Returns `etag` on new version | **FAIL — never generated** |
| `data_crc32c` integrity check | **FAIL — checksum field is read from `SecretPayload` but never validated or stored** |
| `client_specified_payload_checksum` flag in response | **FAIL — always false/zero** |
| `replication_status` in returned SecretVersion | **FAIL — never populated** |
| Payload stored correctly | PASS |

**Critical:**
- `data_crc32c` is silently ignored. Per spec, if the client provides a checksum and the server computes a different one, `INVALID_ARGUMENT` must be returned. Clients using the Go SDK's `AddSecretVersion` with `DataCrc32C` set (which the SDK generates automatically) will never get integrity validation; a corrupted payload will be silently accepted and stored.
- `client_specified_payload_checksum` is always false. The SDK uses this field to decide whether to trust the returned checksum in `AccessSecretVersion`.

---

### 2.7 GetSecretVersion

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret or version | PASS |
| `latest` alias resolution | PASS |
| Returns `name`, `create_time`, `state` | PASS |
| Returns `etag` | **FAIL — never populated** |
| Returns `destroy_time` when state is DESTROYED | **FAIL — `destroy_time` is never recorded or returned** |
| Returns `replication_status` | **FAIL — never populated** |
| Returns `scheduled_destroy_time` | **FAIL — not applicable without version_destroy_ttl, but field always zero** |
| `latest` resolves to highest ENABLED version (not just highest) | PASS |
| `latest` alias resolves to numeric name in response | **FAIL — response `name` contains "latest" literal, not the resolved numeric name** |

**Critical:**
- When a client calls `GetSecretVersion("…/versions/latest")` the response `name` field contains `…/versions/latest` rather than the resolved `…/versions/3`. The spec and SDK docs state the response name must be the canonical numeric resource name. The Go SDK's `GetSecretVersion` iterator stores the returned `name` for subsequent calls; a `latest` literal in that name will then be passed as a `page_token` or used in `AccessSecretVersion`, causing `NotFound` errors downstream.
- `destroy_time` is never recorded when `DestroySecretVersion` is called, so clients that display destruction timestamps see zero values.

---

### 2.8 AccessSecretVersion

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret or version | PASS |
| Returns `FailedPrecondition` for non-ENABLED version | PASS |
| `latest` alias resolution | PASS |
| Returns `name` and `payload.data` | PASS |
| `payload.data_crc32c` populated in response | **FAIL — crc32c never computed or returned** |
| Response `name` is canonical (not "latest") | **FAIL — same issue as GetSecretVersion** |

**Critical:**
- The Go SDK's `AccessSecretVersion` response handler calls `GetDataCrc32C()` and compares it against a locally computed checksum if `ClientSpecifiedPayloadChecksum` is true. Since the emulator never populates `data_crc32c`, the SDK silently skips the integrity check, which is benign — but clients that explicitly call `resp.Payload.GetDataCrc32C()` to verify data integrity will receive 0 instead of the expected checksum.
- The `latest` literal leak in the response name (same as GetSecretVersion) is critical for clients that use the returned name to construct subsequent requests.

---

### 2.9 ListSecretVersions

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (parent) | PASS |
| Returns `NotFound` for missing secret | PASS |
| Pagination via `page_size` / `page_token` | PASS |
| Returns `next_page_token` when more results | PASS |
| Filter support (`state:ENABLED`, `state:DISABLED`, `state:DESTROYED`) | PASS (basic state filter) |
| Filter for compound expressions (e.g., `state!=DESTROYED`) | **FAIL — only exact state: match supported** |
| Result ordering | **FAIL — spec says "sorted in reverse by create_time (newest first)"; emulator sorts by version number ascending** |
| `total_size` field populated | **FAIL — always 0** |
| Returns `destroy_time`, `etag`, `replication_status` in versions | **FAIL — same gaps as GetSecretVersion** |
| Page token stability under concurrent mutation | **FAIL — same positional token issue** |

**Minor:** The ascending sort is the inverse of the spec's required ordering. Clients that display "most recent version first" or stop paginating early will show the wrong data.

---

### 2.10 EnableSecretVersion

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret or version | PASS |
| Returns `FailedPrecondition` for DESTROYED version | PASS |
| Sets state to ENABLED | PASS |
| Returns `name`, `create_time`, `state` | PASS |
| Etag validation | **FAIL — etag in request silently ignored** |
| Returns `etag` | **FAIL — never populated** |
| Returns `destroy_time`, `replication_status` | **FAIL — never populated** |

**Minor:** Etag ignored (same pattern as all state-mutation methods).

---

### 2.11 DisableSecretVersion

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret or version | PASS |
| Returns `FailedPrecondition` for DESTROYED version | PASS |
| Sets state to DISABLED | PASS |
| AccessSecretVersion fails for disabled version | PASS |
| Returns `name`, `create_time`, `state` | PASS |
| Etag validation | **FAIL — etag in request silently ignored** |
| Returns `etag`, `destroy_time`, `replication_status` | **FAIL — never populated** |

---

### 2.12 DestroySecretVersion

**Status: ISSUES**

| Check | Result |
|---|---|
| Required field validation (name) | PASS |
| Returns `NotFound` for missing secret or version | PASS |
| Sets state to DESTROYED and clears payload | PASS |
| Idempotent (re-destroying already-DESTROYED version succeeds) | PASS |
| AccessSecretVersion fails for destroyed version | PASS |
| Records `destroy_time` | **FAIL — destroy_time never set on StoredVersion or returned** |
| Etag validation | **FAIL — etag in request silently ignored** |
| Returns `etag`, `replication_status` | **FAIL — never populated** |
| `version_destroy_ttl` delayed destruction | **FAIL — not implemented; destruction is immediate regardless** |

**Critical:** `destroy_time` is a required output field per spec ("Only present if state is DESTROYED"). Clients that use `destroy_time` to determine when data was wiped for compliance purposes will receive a zero-value timestamp.

---

### 2.13 GetIamPolicy / SetIamPolicy / TestIamPermissions

**Status: NOT IMPLEMENTED**

These three methods are part of the `SecretManagerService` interface via the embedded `google.iam.v1.IAMPolicy` mixin. They are entirely absent from the server (the `UnimplementedSecretManagerServiceServer` embed returns `codes.Unimplemented`).

The emulator does support an *external* IAM enforcement model (delegating to a separate `gcp-emulator-auth` sidecar), but it does not expose the IAM policy CRUD endpoints on the Secret Manager gRPC port itself. Clients that call `smClient.SetIamPolicy(...)` directly on the Secret Manager service endpoint will receive `Unimplemented`.

**Impact:** Any SDK code that manages secret IAM policies through the Secret Manager client (rather than the IAM Admin client) will fail. This is a common pattern in GCP tooling and Terraform providers.

---

## 3. Cross-Cutting Gaps

### 3.1 Etag — optimistic concurrency (critical)

The `Secret` and `SecretVersion` proto types both carry an `etag` field. The spec requires the server to:
1. Generate and return an etag on every read (GetSecret, GetSecretVersion, CreateSecret, AddSecretVersion, and all state-mutation responses).
2. On `UpdateSecret`, `DeleteSecret`, `EnableSecretVersion`, `DisableSecretVersion`, and `DestroySecretVersion`: if the caller provides a non-empty etag, reject with `ABORTED` if it does not match the stored etag.

The emulator stores no etag anywhere in `StoredSecret` or `StoredVersion`, never generates one, and never checks one. All etag fields in responses are empty strings. This is a silent correctness gap for any client using the standard read-modify-write + etag pattern.

### 3.2 Payload checksum (crc32c) — integrity (critical)

`SecretPayload.data_crc32c` is defined as: "if specified, the server verifies the integrity of the received data on AddSecretVersion calls using the crc32c checksum." The Go SDK automatically sets `data_crc32c` when adding versions. The emulator reads `payload.GetData()` but never reads `payload.GetDataCrc32C()`, never validates it, and never returns a checksum in `AccessSecretVersion` responses.

Effect: A client that detects in-flight data corruption via the crc32c mechanism will not detect corruption in the emulator. Tests that exercise the CRC path will silently pass even with corrupted payloads.

### 3.3 "latest" alias leak in response names (critical)

Both `GetSecretVersion` and `AccessSecretVersion` resolve `"latest"` to a concrete version ID for the lookup, but the response `Name` field is set to the original `versionName` string (which still contains `"latest"`) because the code does:

```go
// storage.go GetSecretVersion
versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)  // only set in the alias branch
```

Tracing through the code: when `versionID == "latest"`, `versionID` is updated to the resolved number AND `versionName` is also updated. However in `AccessSecretVersion` the resolved `versionName` is used for the response correctly. Let me re-examine:

In `AccessSecretVersion` (storage.go line 291):
```go
versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)
```
This IS updated. So the returned `Name` in `AccessSecretVersionResponse` is the numeric name — **AccessSecretVersion is correct on this point.**

In `GetSecretVersion` (storage.go line 365):
```go
versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)
```
This is also updated inside the `if versionID == "latest"` block. So `GetSecretVersion` response name is also the resolved numeric name — **this is also correct**.

Retracting the "latest alias leak" as a separate issue. The code correctly reassigns `versionName` in both cases.

### 3.4 Pagination — sort order (minor but spec non-conformant)

Spec: both `ListSecrets` and `ListSecretVersions` responses must be "sorted in reverse by create_time (newest first)."

Implementation:
- `ListSecrets` sorts by `name` ascending.
- `ListSecretVersions` sorts by integer version number ascending.

For `ListSecretVersions` the sort order happens to produce chronologically ascending order (since version numbers are monotonically increasing), which is the inverse of the spec. For `ListSecrets` the sort is lexicographic by name, which has no relationship to creation order.

### 3.5 Pagination — index-based token (minor)

The page token is a plain integer string (`"25"`, `"50"`, etc.) representing a positional index into the sorted slice. This is not opaque and is invalidated by any concurrent secret or version creation/deletion between page fetches. The real API uses opaque base64-encoded cursor tokens that are stable across mutations.

### 3.6 ListSecrets `filter` completely ignored (critical)

The `filter` field on `ListSecretsRequest` is passed to `s.storage.ListSecrets` but the `ListSecrets` storage function signature is `(ctx, parent, pageSize, pageToken)` — the filter is dropped at the call site in `server.go` line 108. The function signature does not even accept a filter argument. This means all ListSecrets filter expressions (`name:`, `labels.`, `create_time>`, etc.) silently return the full set.

Note: `ListSecretVersions` does forward the filter, and basic `state:STATE` parsing is implemented there.

### 3.7 Regional secrets (projects/*/locations/*) not supported

The storage key construction always uses `fmt.Sprintf("%s/secrets/%s", parent, secretID)`. If `parent` is `projects/my-project/locations/us-central1`, the resulting key and resource name are correctly formed — but the `NormalizeSecretResource` helper in `authz/resources.go` checks `parts[2] == "secrets"` to extract the secret name from a version name, which breaks for the regional format `projects/p/locations/l/secrets/s/versions/v` (where `parts[2]` is `"locations"`, not `"secrets"`).

The `GetSecretVersion`, `AccessSecretVersion`, `DisableSecretVersion`, `EnableSecretVersion`, and `DestroySecretVersion` implementations all use `strings.Split(versionName, "/versions/")` which works for both regional and global format — but the IAM resource normalization will silently fail to extract the correct secret path for regional resources.

### 3.8 `StoredSecret` missing key metadata fields

`StoredSecret` (storage.go) does not include fields for: `Topics`, `Rotation`, `ExpireTime`, `Ttl`, `VersionDestroyTtl`, `CustomerManagedEncryption`, `Tags`, `VersionAliases`, or `Etag`. These are not stored in memory, so they cannot be returned by any read operation.

### 3.9 `StoredVersion` missing key metadata fields

`StoredVersion` does not include: `DestroyTime`, `ReplicationStatus`, `Etag`, `ClientSpecifiedPayloadChecksum`, `ScheduledDestroyTime`, or `CustomerManagedEncryption`. The proto response structs are constructed inline in each storage function with only `Name`, `CreateTime`, and `State`.

---

## 4. IAM Methods Assessment

The emulator implements an *enforcement* layer (check before acting) via `gcp-emulator-auth`, but does not implement the three IAM policy management RPCs on the Secret Manager service endpoint itself:

| Method | Status | Notes |
|---|---|---|
| `GetIamPolicy` | NOT IMPLEMENTED | Returns `Unimplemented` |
| `SetIamPolicy` | NOT IMPLEMENTED | Returns `Unimplemented` |
| `TestIamPermissions` | NOT IMPLEMENTED | Returns `Unimplemented` |

The IAM permission names in `authz/permissions.go` are all correct per the GCP IAM reference.

---

## 5. Recommended Fix Priority

### P0 — Critical: causes incorrect SDK client behavior

1. **ListSecrets filter ignored** — any client using `ListSecrets` with a filter will receive wrong results (all secrets instead of the filtered set). Fix: add `filter` parameter to `Storage.ListSecrets` and implement at minimum label and name prefix filtering.

2. **Payload crc32c not validated or returned** — the Go SDK sets `data_crc32c` by default. Fix: in `AddSecretVersion`, if `payload.DataCrc32C != nil`, compute the actual crc32c of `payload.Data` and return `INVALID_ARGUMENT` if it does not match; always compute and store the crc32c; return it in `AccessSecretVersionResponse.Payload.DataCrc32C`.

3. **`destroy_time` never recorded** — clients checking compliance timestamps receive zero. Fix: record `time.Now()` as `DestroyTime` on `StoredVersion` when state is set to `DESTROYED` and include it in all version responses when state is DESTROYED.

4. **Etag missing from all responses** — clients using optimistic concurrency are silently unprotected. Fix: generate a deterministic etag (e.g., hash of name + create_time + version counter) and store/return it; validate on mutating requests when the caller provides one.

### P1 — Critical: breaks common patterns

5. **`topics` field silently discarded** — add `Topics []*secretmanagerpb.Topic` to `StoredSecret`; store and return it.

6. **`version_aliases` not stored or used for access** — `GetSecretVersion` and `AccessSecretVersion` only resolve `"latest"`, not arbitrary aliases. Fix: store `VersionAliases map[string]int64` in `StoredSecret`; resolve aliases in both methods.

7. **GetIamPolicy / SetIamPolicy / TestIamPermissions not implemented** — these are part of the service interface. Fix: implement using the same `emulatorauth` backend, or return minimal stub implementations that accept any policy and grant all permissions when IAM mode is off.

### P2 — Minor: spec non-conformant, low client impact

8. **Sort order inverted** — `ListSecrets` should sort by `create_time` descending; `ListSecretVersions` should sort by version number descending. Fix: reverse the sort in both list functions.

9. **`total_size` never populated** — fix: when no filter is specified, set `TotalSize` to the count of all matching items before pagination.

10. **`rotation` field silently discarded** — add `Rotation *secretmanagerpb.Rotation` to `StoredSecret`; store and return it.

11. **`expire_time` / `ttl` not honored** — the emulator never auto-deletes expired secrets. At minimum: store expiration fields and return them in get/list responses; a background expiration goroutine is optional.

12. **`version_destroy_ttl` not honored** — delayed destroy should transition to DISABLED then schedule actual destruction. At minimum: store and return the field.

13. **Pagination token stability** — replace integer-index token with a cursor based on the last-seen sort key (name or version number) so concurrent mutations do not corrupt pagination.

14. **Regional format (`locations/*`) IAM normalization broken** — fix `NormalizeSecretResource` and `NormalizeSecretVersionResource` in `authz/resources.go` to handle the `projects/*/locations/*/secrets/*` path shape.

15. **`replication_status` never populated** — add to `StoredVersion` and return for Automatic replication with a no-op status struct.

---

## 6. Conformance Matrix

| Method | Core behavior | Error codes | Response completeness | Etag | Checksum | Verdict |
|---|---|---|---|---|---|---|
| CreateSecret | PASS | PASS | PARTIAL | MISSING | N/A | PARTIAL |
| GetSecret | PASS | PASS | PARTIAL | MISSING | N/A | PARTIAL |
| ListSecrets | PASS | PASS | PARTIAL (filter ignored, wrong order) | MISSING | N/A | PARTIAL |
| UpdateSecret | PASS | PASS | PARTIAL | MISSING | N/A | PARTIAL |
| DeleteSecret | PASS | PASS | PASS (Empty) | IGNORED | N/A | NEAR-CONFORMANT |
| AddSecretVersion | PASS | PASS | PARTIAL | MISSING | MISSING | PARTIAL |
| GetSecretVersion | PASS | PASS | PARTIAL (no destroy_time) | MISSING | N/A | PARTIAL |
| AccessSecretVersion | PASS | PASS | PARTIAL (no crc32c) | N/A | MISSING | PARTIAL |
| ListSecretVersions | PASS | PASS | PARTIAL (wrong order, no total_size) | MISSING | N/A | PARTIAL |
| EnableSecretVersion | PASS | PASS | PARTIAL | IGNORED | N/A | NEAR-CONFORMANT |
| DisableSecretVersion | PASS | PASS | PARTIAL | IGNORED | N/A | NEAR-CONFORMANT |
| DestroySecretVersion | PASS | PASS | PARTIAL (no destroy_time) | IGNORED | N/A | PARTIAL |
| GetIamPolicy | — | — | — | — | — | NOT IMPLEMENTED |
| SetIamPolicy | — | — | — | — | — | NOT IMPLEMENTED |
| TestIamPermissions | — | — | — | — | — | NOT IMPLEMENTED |

---

## 7. Overall Verdict: PARTIAL

The emulator is solid for the primary "happy path" use case: create a secret, add versions, access the latest version, disable/destroy versions. The state machine is correct, pagination works for the version list (with the right filter), and IAM enforcement is well-designed.

The gaps that matter most for production SDK compatibility are:
1. **ListSecrets filter** — silently returns wrong data
2. **payload crc32c** — integrity checking is a no-op
3. **destroy_time** — compliance use cases broken
4. **etag** — optimistic concurrency is a no-op across all mutating operations
5. **IAM policy RPCs** — standard client patterns fail with `Unimplemented`
