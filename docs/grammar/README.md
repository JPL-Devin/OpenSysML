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

The grammar files are part of the OMG SysML v2 specification and follow the OMG licensing terms. See the pilot implementation repository for details.

## TODO

- [x] Download and vendor actual grammar files
- [x] Document exact commit hash/release tag
- [ ] Verify grammar version matches stdlib version in use
