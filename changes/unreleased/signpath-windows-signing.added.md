- **Windows Authenticode signing through SignPath Foundation, ready to apply for.** The three
  Windows executables now embed a `VERSIONINFO` resource (`ProductName` `OpenSysML`,
  `ProductVersion` and `FileVersion` taken from the same `VERSION` the `-ldflags` carry,
  `CompanyName`, `FileDescription`, `LegalCopyright`, `OriginalFilename`), written by a pinned
  `go-winres` for `GOOS=windows` builds only and checked against the tag by
  `make windows-versioninfo-check`. A new GitHub Actions workflow, `release-windows.yml`,
  rebuilds them on every `v*` tag with the CircleCI release job's Makefile targets and version
  variables, submits them to SignPath for signing once the `SIGNPATH_API_TOKEN` secret and the
  `SIGNPATH_*` variables are configured — and otherwise stops after the build — and publishes the
  signed files as additional `*-signed*` release assets with a `SHA256SUMS-windows-signed.txt`,
  leaving the CircleCI-published assets, `SHA256SUMS.txt` and its cosign bundle untouched. The
  README gains the "Code signing policy" section SignPath's terms require (team roles, MFA,
  privacy statement), and the releasing guide the application, configuration and per-release
  approval procedure. Signing takes effect only after the project's SignPath application is
  approved.
