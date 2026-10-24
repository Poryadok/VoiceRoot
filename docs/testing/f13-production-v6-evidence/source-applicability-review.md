# Historical patch rendering source applicability

This author assessment covers exactly the two files in H
5a817a37e145c10d3b2b87b03454d777e36b10c4 above
122bcedd7757139d441bc7b40220d98e21b117c4. It is not independent approval.
The helper adds only `--abbrev=9` to the existing historical diff command.
All 243 artifact hashes, eight merge tuples, 38 occurrences, full object entries,
record bytes, source/current-target constraints and retained runtime obligations
remain unchanged. The test fixture freezes the same nine-digit format and adds
hostile default widths plus a modeled expanded-return rejection. Git may expand
a prefix on a collision; the unchanged digest rejects those bytes rather than
truncating or accepting them.

The actual producer failed on a 1552-byte diff whose two index IDs had ten digits.
The original expected 1550-byte diff is reproduced exactly with the explicit
format. The failure and byte comparison are retained. The original qualifier
reconstructed all 243 artifacts successfully in the failed object database on a
clean private candidate commit, under default settings and widths 7, 10 and 40.
These results prove source qualification, not full producer completion.

An AST audit of all F13 Python Git diff calls and CI Python/shell patch generators
found that other fixed textual patch predicates already use `--abbrev=9`:
exact-J edges, finite-J source edges and the fixed3071 whole patch. Raw tree and
inventory predicates use `--no-abbrev`. Two remaining F54 `--binary` calls create
fresh corrected-F/manual-F receipt digests; they do not compare an immutable
archived patch digest. No speculative normalization was added there.

Every pre-existing Git tree entry outside these two files is unchanged, including
the Native projection/context/directory inventories, Role unit, corrected-F unit,
all predecessor source records and evidence. The historical helper stays outside
the ten executable carrier paths. This version binds its exact reviewed bytes;
it does not permit future edits to this helper or other scripts. The original
producer/router still evaluates every contribution and current obligation.
