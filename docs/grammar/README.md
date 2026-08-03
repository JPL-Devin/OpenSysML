# SysML v2 Reference Grammar

This directory contains vendored copies of the official SysML v2 grammar files from the OMG pilot implementation.

## Source

**Repository:** https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation

**Version:** 2026-05 Release

**Commit:** 4c289b926 (2026-07-24)

**Grammar Files Location:**
- KerML: `org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext`
- SysML: `org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext`

## Files

- `KerML.xtext` - Kernel Modeling Language grammar (base layer) - 28KB
- `SysML.xtext` - Systems Modeling Language grammar (extends KerML) - 61KB

Vendored from commit 4c289b926 (2026-07-24).

## Usage

These grammar files serve as the **authoritative source of truth** for parser implementation and verification. See `PRODUCTION_MAP.md` for mapping between grammar productions and Go parser functions.

## License

**Eclipse Public License v2.0 (EPL-2.0)**

The grammar files retain their original EPL-2.0 license from the SysML v2 Pilot Implementation:

```
Copyright (c) 2018-2025 Model Driven Solutions, Inc.
Copyright (c) 2018 IncQuery Labs Ltd.
Copyright (c) 2019 Maplesoft (Waterloo Maple, Inc.)
Copyright (c) 2019 Mgnite Inc.

This program is free software: you can redistribute it and/or modify
it under the terms of the Eclipse Public License as published by
the Eclipse Foundation, version 2 of the License.
```

Full license text: https://www.eclipse.org/legal/epl-2.0/

**SPDX-License-Identifier:** EPL-2.0

These files are vendored unmodified from the upstream repository for reference purposes only. Our Go parser implementation is independent and licensed under Apache 2.0 (see project root LICENSE).

## TODO

- [x] Download and vendor actual grammar files
- [x] Document exact commit hash/release tag
- [ ] Verify grammar version matches stdlib version in use
