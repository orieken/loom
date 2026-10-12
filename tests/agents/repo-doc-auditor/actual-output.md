{
  "schemaVersion": 2,
  "path": "internal/ledger",
  "inputHash": "sha256:4be1d0a9c37f2e85b6a01d7c9e3f48a2d5c60b17e9f3a4d8c2b5e1f07a96d3c4",
  "verdict": "CHANGES_REQUESTED",
  "findings": [
    {
      "claim": "Cryptographically signs every `Record` with a per-tenant key",
      "problem": "Cites exports:Digest and exports:Record. An export named `Digest` supports at most \"computes a digest\" \u2014 nothing in the fact sheet mentions cryptographic signing, keys, or tenants, and those are mechanisms a name cannot carry. The claim also asserts totality (\"every\") that the cited fields do not establish.",
      "instruction": "Soften to what `Digest` actually supports (e.g. \"computes a digest for a `Record`\"), or move the signing/per-tenant-key detail to openQuestions since the fact sheet gives no field that supports it.",
      "blocking": true
    }
  ]
}
