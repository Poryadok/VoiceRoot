# Catalog reference expiry after authority locks

This is the narrow correction for reviewer140's P2 on exact
`e63d5d55416d702dc0a6a4b8e70b38b43b498b18`. It has no derived-GIF or
migration13 dependency.

The locked catalog reader observes PostgreSQL `clock_timestamp()` after its
File, blob and selection-head lock waits, immediately before the READY/expiry
predicate. A caller clock captured before the transaction, or PostgreSQL
transaction-start `now()`, cannot authorize an already expired File.

The causal PostgreSQL test holds File UPDATE, starts the actual protected
Acquire handler and observes it blocked. It installs an expiry later than the
already captured caller clock, waits for actual database expiry, then releases
the lock. The old reader incorrectly acquired a reference (RED); the corrected
reader rejects it and commits neither a reference nor an operation receipt.
The reference matrix passed race46.700s; File short, scoped vet and diff checks
passed. Ordinary lifecycle behavior and exact completed-operation replay remain
unchanged. Independent delta review is still required before closing the P2.
