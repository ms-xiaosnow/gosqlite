# Third-party notices

No third-party implementation code is copied into the production packages at M0. The Go standard library/toolchain uses the [Go BSD-style license](https://go.dev/LICENSE). SQLite 3.54.0 public sources are used outside production solely as behavior references and a test oracle; see the [SQLite public-domain dedication](https://www.sqlite.org/copyright.html). Oracle build dependencies/tools are not production dependencies.

The specification links pgvector, pg_bigm, Neon and public SEE documentation. These links confer no permission to copy restricted implementations; no SEE private source is accessed. Any later vendored/copied dependency requires exact version and license review here. Independent pre-release legal/license review remains NOT_RUN.


## golang.org/x/text v0.43.0

Production Unicode normalization/casefold dependency for grams13, explicitly permitted by the original specification. Official Go Authors source at commit ac40b0f06c159afb5bfca4d1860f8b4b60dbc160. Copyright 2009 The Go Authors. BSD-3-Clause; exact license retained in [third_party/golang.org-x-text-LICENSE](third_party/golang.org-x-text-LICENSE). Module is pinned in go.mod/go.sum and downloaded through the official proxy with Go checksum-database verification. This is an informational source/license check, not the independent release legal audit required by M10. Its development/tool requirements are not automatically production imports; the actual production package closure is checked separately.

## pgvector v0.8.7 published test fixtures

Public semantic reference pinned to commit f37c13f68b57d2c3472b2214fbcff699d6d34876 at https://github.com/pgvector/pgvector. The unmodified test/sql/vector_type.sql and test/expected/vector_type.out are retained under testdata/pgvector-v0.8.7 with hashes and an independently extracted golden subset. The upstream notice credits portions Copyright (c) 1996–2026 PostgreSQL Global Development Group and portions Copyright (c) 1994 The Regents of the University of California; the exact PostgreSQL-style license is retained in third_party/pgvector-LICENSE. Production vector code is independently written Go; no pgvector C implementation is copied, compiled or invoked. This informational provenance/license check is not the independent M10 legal audit.

## golang.org/x/crypto v0.58.0 and golang.org/x/sys v0.49.0

Experimental memory-only authenticated-record primitives use official Go Authors
Argon2id from x/crypto at commit8fdf07b3114278fb3b78c1cf1814e08bd851eb4c.
Its actual package closure imports blake2b and x/sys/cpu; x/sys is pinned to
commitb6b8557e8e2974925aec7a136ba0e323eedc5f43. Both are BSD-3-Clause; exact
notices retained in third_party/golang.org-x-crypto-LICENSE and
third_party/golang.org-x-sys-LICENSE. Official proxy downloads were independently
SHA256-hashed and verified through Go's checksum database; go.mod/go.sum pin
versions. No CgoFiles exist in the imported closure. Other optional dependencies
in upstream go.mod are not imported into this project. This informational
provenance/license check is not independent legal or cryptographic acceptance.
