# Third-party notices

No third-party implementation code is copied into the production packages at M0. The Go standard library/toolchain uses the [Go BSD-style license](https://go.dev/LICENSE). SQLite 3.54.0 public sources are used outside production solely as behavior references and a test oracle; see the [SQLite public-domain dedication](https://www.sqlite.org/copyright.html). Oracle build dependencies/tools are not production dependencies.

The specification links pgvector, pg_bigm, Neon and public SEE documentation. These links confer no permission to copy restricted implementations; no SEE private source is accessed. Any later vendored/copied dependency requires exact version and license review here. Independent pre-release legal/license review remains NOT_RUN.


## golang.org/x/text v0.43.0

Production Unicode normalization/casefold dependency for grams13, explicitly permitted by the original specification. Official Go Authors source at commit ac40b0f06c159afb5bfca4d1860f8b4b60dbc160. Copyright 2009 The Go Authors. BSD-3-Clause; exact license retained in [third_party/golang.org-x-text-LICENSE](third_party/golang.org-x-text-LICENSE). Module is pinned in go.mod/go.sum and downloaded through the official proxy with Go checksum-database verification. This is an informational source/license check, not the independent release legal audit required by M10. Its development/tool requirements are not automatically production imports; the actual production package closure is checked separately.
