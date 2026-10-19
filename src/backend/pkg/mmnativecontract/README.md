# Native MM database observations

`ObserveDatabase` is a read-only fact collector shared by the native application
and an independently authorized installation driver. It issues no receipt,
changes no role/resource and establishes no installed authority.

Database binding SHA-256 grammar: domain `voice.mm.database-binding.v1` plus NUL,
big-endian U32 version1, U32-length-prefixed canonical decimal system identifier,
database OID as U32, U32-length-prefixed database name. The system identifier is
read from actual `pg_catalog.pg_control_system()`; the login needs EXECUTE on
that exact metadata function, not an admin role or SET ROLE bridge.

Schema SHA-256 grammar: domain `voice.mm.public-schema-contract.v1` plus NUL,
big-endian PostgreSQL server-version number, then the UTF-8 PostgreSQL JSONB text
of the code-owned catalogue query. It covers public relation/view/partition
definitions, columns/defaults, constraints, indexes, trigger enablement,
functions and sequences. Runtime rows/sequence positions and object OIDs are
excluded. Login and ACL closure are checked separately by the native root.
Native runtime rejects public row-level-security configurations it does not
implement. This is a required MM schema contract, not a universal database dump.

Two databases on the same PostgreSQL system have different database bindings.
A copied/restored PostgreSQL system identifier is still insufficient to prove
continuity: independent controller lineage, immutable receipts, credential and
launch custody remain mandatory. The caller reads with its local admission
gate held; a canonical hash or a fixture row cannot substitute for that custody.
