- **Windows releases ship an installer, `opensysml-<x.y.z>-windows-amd64.msi`.** A WiX v5 MSI
  (`packaging/msi`, `scripts/build-msi.sh`) installs `sysml.exe`, `sysml-lsp.exe` and
  `sysml-grpc.exe` to `Program Files\OpenSysML` on the system `PATH`, upgrades an older
  install in place, and offers the Z3 SMT solver (5.1.0, pinned by SHA256 in
  `packaging/msi/z3.pin`, MIT notice included) as an optional feature under `z3\` on `PATH`,
  so `%check`/`%explain` find a solver without configuration. The release workflow publishes it
  unsigned with `SHA256SUMS-windows-msi.txt`, or — once SignPath is configured — built from the
  signed executables and itself signed as `*-signed.msi` (the bundled `z3.exe` is never
  signed). The install guide recommends the MSI on Windows.
- **Scoop, winget and MSYS2 manifests are maintained in-repo as templates.**
  `packaging/scoop`, `packaging/winget` and `packaging/msys2` hold manifests that depend on the
  package manager's Z3 instead of bundling it, rendered from a release by
  `scripts/render-scoop-manifest.sh`, `scripts/render-winget-manifests.sh` and
  `scripts/render-msys2-pkgbuild.sh` like the Homebrew formula; each README documents how a
  maintainer submits to the external bucket or repository. Nothing is submitted automatically.
