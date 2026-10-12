# R5 model-name integration correction: narrow independent review

**CLEAR for final candidate `8751a3bed18341a299c060ff494c437d5176b024`.** No remaining blocking defect found in this bounded correction. Earlier source/physical limitations remain unchanged.

`SOURCE-IDENTITY.json` SHA-256: `b3d49c32f75546ae7cbbedcc69891047506133a9cd10b4613f4f65d17e29fb5b`. All 20 current file hashes and their corresponding candidate-tree blobs were independently compared and match.

## Finding and final correction

The initial correction accounted for four protected `SQLPlan` identifiers but left three protected `SQLQueryText` identifiers. I reported that remaining blocker before execution: the unchanged rollback SQL guard protects both names, accounting exactly for all seven original sourceguard diagnostics. The final correction now renames only these new R5 names:

- `SQLPlan` to `RekeyQueryPlan`: two model declaration/use occurrences and two test literals.
- `SQLQueryText` to `RekeyQueryText`: the model function declaration and one call, plus one test call.

Independent byte comparison confirms that each model file is exactly its preserved before file with those substitutions. Field names, field order, underlying types, JSON-visible shape, constants, SQL strings, function bodies and validation decisions are unchanged. Neither renamed identifier is serialized as a type/function name by this value model. No native source references to these model names needed changes.

The R5 guard changes only the two affected source pins, two exact source snapshots and the model function-inventory spelling. All nine pins still match their actual source files, all nine snapshots equal those files byte-for-byte, and all 36 independent critical body literals remain unchanged. The guard's code outside the supplemental maps and that one inventory spelling is byte-identical to the preserved before source.

The existing integration test adds two positive calls to the unchanged `inspectRollbackFaultSQLSource`, one for each R5 model fixture, requiring no error or finding. Existing negative checks and test roots remain. The old rollback SQL guard is byte-identical to base commit `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`; SHA-256 `0647edf67ae831b8d99b3caddf02fbf8e65380fc391405effe433145c5374969`. No old guard, exception, production implementation, native controller, bridge or ordinary-main source is weakened or changed.

## Final changed-source identities

- contract.go: `e834e098a978d1715189e3cee293aab62ea2b806d27f892a60b3d6f4d89f888e`
- contract_test.go: `41f7343636c45b56f081875db9ab0b81273bbef212282a02a46f6ae1e5978cc6`
- rekey_interruption.go: `9bab65139005fd2a296f8db023528613c41df94860d60304ad1564f073f7c6a8`
- rekey_interruption_test.go: `f562a86ac3c986561a21f219fc5eb0d7793546b7732594dd781897b72913cc3a`

## Bounded continuation

V3 runner source SHA-256: `6c25a77cf3a95410e9ab3ffe63c2acc3a98ca31ea8f0480acc5676b95c32d5b3`. Its reviewed plan checks formatting of these four files, runs only seven R5 AST roots normal/race and eight model roots normal/race, reruns the failed sourceguard, then runs the five previously unrun source-selection/compile/nm commands. The updated model hashes and identity-release location match this candidate. The 54 existing guard roots and two healthy default roots are reused as specified; no manual test image is executed. Earlier failures remain evidence, not rewritten PASS results.

This reviewer performed source/diff/hash and existing-evidence reads only and wrote this report. No Go, gofmt, parser/scanner executable, compiler, test, sourceguard or native operation was run; no source, pin, runner or release file was edited. Actual V3 results remain pending.
