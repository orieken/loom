---
feature: "user-avatar-upload"
bounded_context: "users"
domain_terms: ["avatar", "StorageAdapter"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

<!-- FIXTURE MODE: this repository has no README.md, CHANGELOG.md, runbook, or API docs for this feature.
No files were written. Below is the exact content this agent would add, and where. -->

# Documentation Report: User Avatar Upload

## Files Updated
- `CHANGELOG.md` — new `[Unreleased]` entry:
  ```markdown
  ### Added
  - Users can now upload a profile avatar. The system rejects files that
    exceed the size limit or use an unsupported format.
  ```
- `README.md` — new subsection under the existing "Features" or "User Profile" heading:
  ```markdown
  ### Avatar Upload

  Users can upload a profile avatar from their account settings. The system
  stores the file in S3 and replaces any existing avatar.

  Supported formats and the maximum file size follow the limits enforced by
  `upload-handler.ts`. An upload that exceeds the size limit or uses an
  unsupported format fails with a validation error. The system processes one
  upload per user at a time; a concurrent upload from the same user waits
  for the first upload to finish.
  ```
- `docs/api/storage-adapter.md` (new file) — API reference for the new `StorageAdapter` interface:
  ```markdown
  # StorageAdapter

  `StorageAdapter` is the interface the avatar upload handler uses to store
  and remove files. `s3-adapter.ts` implements it for S3.

  ## Methods

  ### `upload(key: string, body: Buffer, contentType: string): Promise<void>`
  Stores `body` at `key`. The call MUST NOT update the user record until
  the upload succeeds — on failure, the handler rolls back and leaves the
  previous avatar in place.

  ### `delete(key: string): Promise<void>`
  Removes the object at `key`. The handler calls this for the previous
  avatar's key after a new upload succeeds.
  ```
- `docs/runbook.md` — new "Environment Variables" entry (or a new file if no runbook exists yet):
  ```markdown
  ## Avatar Upload

  Avatar upload requires these environment variables:

  | Variable         | Description                              |
  |------------------|-------------------------------------------|
  | `AWS_S3_BUCKET`  | Name of the S3 bucket that stores avatars |
  | `AWS_REGION`     | AWS region of that bucket                 |

  Set both variables before you deploy this feature. The service fails to
  start the upload handler without them.
  ```
- `docs/adr/ADR-00X-avatar-storage-s3-adapter.md` (new file) — records the decision to introduce the `StorageAdapter` interface rather than call the AWS SDK directly from the upload handler:
  ```markdown
  # ADR-00X: StorageAdapter interface for avatar storage

  Date: 2026-10-11
  Status: Accepted

  ## Context
  The avatar upload handler needs to store and delete files in S3. Calling
  the AWS SDK directly from the handler would couple the handler to S3 and
  make the upload and rollback logic hard to test without a live bucket.

  ## Decision
  Introduce a `StorageAdapter` interface with `upload` and `delete` methods.
  `s3-adapter.ts` implements it for S3. The upload handler depends only on
  the interface.

  ## Consequences
  Unit tests mock `StorageAdapter` instead of the AWS SDK, which is how the
  S3-failure-rollback and old-key-deletion tests in `upload-handler.spec.ts`
  work without a live bucket. Adding a different storage backend later means
  writing a new adapter, not changing the handler.
  ```

## Files Unchanged (and why)
- `docs/setup.md` — no change. The feature adds two environment variables, documented in the runbook entry above, not new local setup steps.
- `docs/migrations.md` — no change. QA's report lists no database schema changes beyond the existing avatar columns on the user record.

## Notes for DevOps
- `AWS_S3_BUCKET` and `AWS_REGION` are new required environment variables. Add them to every deployment environment before this feature ships.
- QA flagged a recommended, not-yet-implemented ops step: a 30-day S3 lifecycle rule on the `avatars/` prefix, to clean up orphaned objects from failed or superseded uploads. This is not part of this feature's code and needs a separate infrastructure task.
