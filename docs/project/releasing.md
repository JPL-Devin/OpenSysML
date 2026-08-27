# Releasing OpenSysML

A release is cut by pushing a `v*` tag. Everything after that is CircleCI: the
`release` workflow runs the test suite, cross-compiles `sysml`, `sysml-lsp` and
`sysml-grpc` for five platforms, and publishes them to a GitHub release. Nothing
is published from a laptop.

The Python client is released separately, by a `opensysml-v*` tag, which runs the
`release-python` workflow and uploads `opensysml` to PyPI — see
[Releasing opensysml to PyPI](#releasing-opensysml-to-pypi). The two are independent
on purpose: `opensysml` resolves a `sysml-grpc` binary at runtime from whichever
release the caller names, not from a release matching its own version.

A third tag, `pysysml-v*`, publishes the one-off final release of the client's
pre-rename PyPI name — see [The final `pysysml` release](#the-final-pysysml-release).

## Before tagging

Run the full gate on the commit you intend to tag:

```bash
gofmt -l .            # must print nothing
go build ./...
go vet ./...
make lint             # staticcheck + gosec, as CircleCI runs
go test -race -count=1 ./...
go test -run TestStdlibConformance ./internal/core/libs
```

Run the Python client the way CircleCI's `python-test` job does, since a
`opensysml-v*` release gates on the same suite:

```bash
make build-grpc && mkdir -p ~/.opensysml/bin && cp bin/sysml-grpc ~/.opensysml/bin/
pip install -e python/ && pip install pytest pytest-mock
pytest python/tests/ -v
```

The OMG training-corpus gate skips while the corpus is absent, so fetch it and
run it explicitly — the expected result is the pinned baseline, currently
100/100 files clean:

```bash
./scripts/download-training-examples.sh
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 go test -count=1 ./internal/core/model -run TestTrainingExamples
```

A change in that count is a finding to adjudicate file by file, never a
baseline to regenerate.

Then check the release-facing text:

- `CHANGELOG.md` has an entry for this version, dated, with the previous
  version's entry unchanged.
- `README.md` and `docs/guide/` transcripts match what the binary prints.
  Build it (`make build-sysml`) and paste a few commands through it.
- `python3 scripts/check-doc-links.py` reports no broken link (CI gates on it too).
- Test counts match a real run and agree across the four surfaces allowed to repeat them
  (`docs/project/spec-compliance.md`, `README.md`, `docs/project/roadmap.md`,
  `docs/project/training-examples.md` — everything else links to the first, per CONTRIBUTING.md),
  and no compliance row claims more than the implementation does. Count first-level subtests:
  a case that registers sub-subtests, like `variant_connection_per_owner`, otherwise counts twice.

## Tagging

The tag is the version: CircleCI passes `CIRCLE_TAG` to the build as
`VERSION`, so `sysml --version` reports it.

```bash
git checkout main && git pull
git tag -a v0.0.5 -m "v0.0.5"
git push origin v0.0.5
```

The tag belongs on the repository the releases live on. v0.0.1–v0.0.7 are
releases of `Open-MBEE/OpenSysML`, while development happens on
`JPL-Devin/OpenSysML`, which has no tags at all — so cutting a release means
promoting `main` upstream first (v0.0.4 came through Open-MBEE PR #47) and
tagging there. Tagging the development repository would build a release nobody
consumes.

Tags are matched by `/^v.*/` in `.circleci/config.yml`. A tag on a commit that
fails the suite fails the release workflow before anything is published.

## What CircleCI publishes

`build-release` produces, in `dist/`:

- per-binary archives — `sysml-<os>-<arch>.tar.gz`,
  `sysml-lsp-<os>-<arch>.tar.gz` (`.zip` on Windows);
- bundle archives — `opensysml-<os>-<arch>.tar.gz` holding both binaries under
  their plain names, which is the layout Homebrew and a PATH install expect;
- `sysml-grpc-<os>-<arch>`, published raw with a `.sha256` sidecar rather than
  archived, because that is what `opensysml` downloads and verifies
  (`python/opensysml/binary.py`) when it starts the service for a Python caller;
- `SHA256SUMS.txt` over every archive and every `sysml-grpc` binary.

Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
windows/amd64.

Before any of it is stored or published, `build-release` runs each host-platform
binary and fails the release unless `--version` reports `CIRCLE_TAG`. The ldflags
are the only thing stamping the tag into a binary, and a binary reporting `dev`
or a stale tag looks the same on the release page as a correct one — that is how
an artifact whose version disagreed with its tag reached a release once already.
The check runs the linux/amd64 builds; the cross-compiled ones cannot run on the
executor, so each is checked for the tag string the ldflags write into it.

`publish-github-release` uploads them with `ghr`, using a token from
`GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project settings.
It runs with `-replace`, so re-running the workflow for the same tag replaces
that release's assets rather than appending duplicates, and leaves everything
else on the release alone: notes, title and the prerelease/latest flags survive.
A tag that has no release yet still gets one created.

Do not go back to `-delete`. It is an alias of `-recreate`: it deletes the
existing release *and its tag* and creates an empty one, which wipes
hand-written release notes (the notes must therefore be on a published release —
`ghr` does not see a draft release for the tag and would publish a second, empty
one alongside it).

## After the release

1. **Verify a download** on at least one platform:

   ```bash
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/opensysml-linux-amd64.tar.gz
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/SHA256SUMS.txt
   sha256sum -c SHA256SUMS.txt --ignore-missing
   tar xzf opensysml-linux-amd64.tar.gz && ./sysml --version
   ```

   `--version` must report the tag, not `dev`.

   Then check the path `opensysml` takes, since it reads the sidecar rather than
   `SHA256SUMS.txt`:

   ```bash
   OPENSYSML_GITHUB_REPO=Open-MBEE/OpenSysML python -c \
     "from opensysml.binary import download_binary; print(download_binary('latest'))"
   ~/.opensysml/bin/sysml-grpc -version
   ```

   A checksum mismatch there means the sidecar and the binary came from
   different builds.

   For a release no published `opensysml` pins, that path depends on the
   signature, so check it the way the client does:

   ```bash
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/SHA256SUMS.txt.bundle
   cosign verify-blob SHA256SUMS.txt --bundle SHA256SUMS.txt.bundle \
     --certificate-oidc-issuer https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62 \
     --certificate-identity-regexp '^https://circleci\.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/[0-9a-f-]+$'
   ```

   A missing bundle means `build-release` did not sign — re-run the tag's
   workflow rather than pinning around it.

2. **Let the Homebrew tap pick the release up.** The tap repository
   `Open-MBEE/homebrew-tap` updates itself: a scheduled workflow there resolves
   the latest `Open-MBEE/OpenSysML` release, renders `Formula/opensysml.rb` from
   this repository's `scripts/render-homebrew-formula.sh` and formula template at
   that tag, and commits only when the file changed. Nothing here triggers it, so
   the formula follows the release within the workflow's schedule interval.

   If it does not, check the workflow run in the tap repository. The render reads
   the release's `SHA256SUMS.txt`, so a release missing that asset (or missing a
   `opensysml-<os>-<arch>.tar.gz` line in it) fails the run loudly instead of
   committing a broken formula — re-run `publish-github-release` for the tag and
   then the tap workflow (`workflow_dispatch`). Rendering by hand still works:

   ```bash
   scripts/render-homebrew-formula.sh v0.0.5 > Formula/opensysml.rb
   ```

   See [packaging/homebrew/README.md](../../packaging/homebrew/README.md).

3. **Say what is not signed.** macOS binaries are not Developer ID signed or
   notarized and Windows binaries are not Authenticode signed, so a browser
   download trips Gatekeeper or SmartScreen. Point release notes at
   [MACOS_DISTRIBUTION.md](macos-distribution.md), which gives the workarounds
   and what signing would take.

### The signed checksum manifest

`build-release` signs `dist/SHA256SUMS.txt` — the manifest covering every
published artifact, including each `sysml-grpc-<os>-<arch>` — with cosign
keyless, and `publish-github-release` uploads the sigstore bundle beside it as
`SHA256SUMS.txt.bundle`. The certificate identity comes from the job's CircleCI
OIDC token exchanged with Fulcio, so **no signing key exists anywhere**: nothing
to provision, rotate or leak. The job then verifies its own bundle and fails the
release rather than publish a signature clients would reject.

That signature is what lets `opensysml` install a core release published after
it. For a release it pins no digest for, the client downloads the manifest and
the bundle, verifies the bundle, and takes the asset's digest from the verified
manifest. The only signature it accepts is this pipeline's:

| | |
|---|---|
| OIDC issuer | `https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62` |
| Certificate subject | `https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/<pipeline definition>` |

The issuer is CircleCI's OIDC issuer for the organization that owns this
project, and the subject is the pipeline definition that ran the job, which is
what CircleCI puts in the certificate. The client currently accepts any pipeline
definition of that project, because no signature of the real one exists to read
the identifier off yet — the verify step in `build-release` prints it, so after
the first signed release set it as `definition=` on the signer in
`python/opensysml/signing.py` to narrow the pin to the one pipeline.

Anything short of a verified manifest is refused exactly as an unpinned release
is today: no bundle asset, a bundle that does not verify, another signer, a
manifest changed after signing, an expired certificate, or `sigstore` not
installed. The `.sha256` served beside a binary is still never a reason to trust
it — same origin as the binary — and remains behind
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`.

### Pinned release digests

`PINNED_SHA256` in `python/opensysml/binary.py` still covers the releases
published before signing existed, and it stays the override: where a pin exists
it wins, and a verified manifest that disagrees with a pin is an error rather
than a downgrade. **Per release there is now nothing to do** — pinning a release
signed by the pipeline is optional. Pinning still works, and is worth doing for
a release clients on an older `opensysml` should be able to install:

```bash
export GITHUB_TOKEN=...   # must be able to read this repository's releases
python python/scripts/pin_release_checksums.py --version v0.0.8 --write
```

The token is required, not an optimization: the script reads the release's assets
through the GitHub releases API, and unauthenticated calls are rate-limited per
address and fail as an opaque HTTP 403. `GH_TOKEN` is read as well. The scope
needed is read access to this repository's releases — `public_repo` for a classic
token, `Contents: read` for a fine-grained one; nothing is written through the
API. Without either variable the script fails immediately with
`MissingTokenError` naming the variable, rather than at the first request.

## The SonarCloud scan

Not a release step — the `scan` job runs in the `build-test` workflow on every
commit, after `build-and-test` — but it is documented here with the other
CircleCI credential plumbing.

The job references the organization context named exactly `SonarCloud`, which
supplies `SONAR_TOKEN` (the same context `Open-MBEE/flexo-mms-layer1-service`
uses, so no new credential is provisioned). It reads
`sonar-project.properties` at the repository root and the `coverage.txt`
profile that `build-and-test` writes (`go test -coverprofile=coverage.txt`) and
persists to the workspace, and it un-shallows the clone because SonarCloud
needs full history for blame and new-code detection.

On a forked PR the context is withheld, so `SONAR_TOKEN` is empty; the job
halts successfully rather than failing every outside contribution. When the
token is present, a failing scan fails the job.

One-time maintainer step (already done for `Open-MBEE_OpenSysML`, but true of
any future project): SonarCloud does not create a project from a CI-run scan
(the scanner sends branch parameters, and Cloud cannot provision from those —
the first run fails with `Could not find a default branch for project with key
'...'`). Create the project under the organization first, either from the
SonarCloud UI or with `POST api/projects/create` followed by
`POST api/project_branches/rename`, using a token that has Create Projects in
that organization.

## Releasing opensysml to PyPI

The Python client in `python/` is published to PyPI as
[`opensysml`](https://pypi.org/project/opensysml/) by the `release-python` workflow,
which runs on a tag matching `/^opensysml-v.*/` — for example `opensysml-v0.3.0`.
The first release under the new name is 0.3.0: the version line carries on from
`pysysml` 0.2.0, which was the same client, so no version number is reused.
Nothing is uploaded from a laptop, and a `v*` core release tag publishes no
package.

### Why its own tag

`opensysml` does not ship the service: it downloads a `sysml-grpc` binary at
runtime for whatever release the caller names (`version=`,
`$OPENSYSML_GRPC_VERSION`, or `latest`), verifying it against the digest it pins
for that release (`PINNED_SHA256` in `python/opensysml/binary.py`) or, for a
release it pins nothing for, against the digest in the release's signed
`SHA256SUMS.txt` (see [the signed checksum manifest](#the-signed-checksum-manifest)),
which is why a core release published after a client release needs no new client
release. Its version therefore says nothing about which core release it runs
against, and tying the two together would put a new, immutable PyPI version on
every core release and would block a client-only fix behind a core release.

Keeping them apart also protects the `v*` path: `publish-github-release` runs
`ghr -replace`, so re-running a core release is an ordinary operation, while a
PyPI version can be yanked but never re-uploaded. A re-run must never have an
irreversible upload hanging off it.

### The version, in one place

`python/opensysml/_version.py` is the only declaration:

- `python/pyproject.toml` has `dynamic = ["version"]` and reads
  `opensysml._version.VERSION` (there is no `setup.py` any more —
  `pyproject.toml` declares the build);
- `opensysml.__version__` reports that declaration, which ships beside the module
  and is therefore the version of the code being imported. A wheel's metadata is
  generated from it, so the two agree there; an editable install's dist-info is
  written once, at install time, and a checkout that bumps `VERSION` afterwards
  would otherwise report the version it had when `pip install -e` ran.

`python/tests/test_version.py` fails if a second version literal reappears
anywhere under `python/`, or if the declaration, the installed metadata and
`__version__` stop agreeing. Where the install is editable, the tests locate the
package through the install's own PEP 610 record (`opensysml/_dist.py`) rather than
the dist-info's directory, which for an editable install is a site-packages path
holding no `opensysml/` at all.

The tag must name the declared version. `python/scripts/check_version.py` is run
by the job before anything is built, and fails loudly otherwise:

```bash
python python/scripts/check_version.py --tag opensysml-v0.3.0   # prints 0.3.0
```

So a release is: bump `VERSION` in `python/opensysml/_version.py`, land it, then
tag `opensysml-v<that version>`.

### What the job needs

The token lives in a **restricted context**, not in project environment
variables, so only the release path can read it:

1. In CircleCI, **Organization Settings → Contexts**, in the context named
   `PyPI` (create it if the organization does not have it yet). A context
   reference in the config is matched exactly, so the name must be spelled with
   the same case in both places.
2. **Restrict it to a security group** (Contexts → `PyPI` → *Add security
   group*) so only that group's members can run a job that uses it. A context
   with no group restriction is readable by every project job.
3. Add the token as `PYPI_API_TOKEN` (an *environment variable* in that
   context). `TWINE_USERNAME` is `__token__`, set by the job; only the token
   value belongs in the context.
4. Optionally add `TEST_PYPI_API_TOKEN`, a TestPyPI token, which is what a
   pre-release tag uses (see the dry run below).

`.circleci/config.yml` references the context from the job in the workflow:

```yaml
      - publish-pypi:
          context:
            - PyPI
```

Any other variables that context happens to carry are ignored. In particular a
`PYPI_USERNAME`/`PYPI_PASSWORD` pair cannot publish to PyPI at all: uploads from
an account with 2FA have required an API token or a trusted publisher since
2023-06-01, and 2FA has been mandatory for every account since 2024-01-01, so a
password is answered with a 403.

The job refuses to run `twine` when the variable it needs is absent, naming the
variable and the context, rather than letting PyPI answer with a 403 that reads
like a permissions problem. It never echoes the token and never prints the
environment.

### The token the job uses

`opensysml` exists on PyPI (0.3.0 and 0.3.1 are published), so the token in the
`PyPI` context as `PYPI_API_TOKEN` must be **scoped to the `opensysml` project**,
never account-scoped: an account-scoped token in CI can publish anything the
account owns. Keep a second owner/maintainer on the PyPI project as well, so it is
not tied to one account.

PyPI trusted publishing (OIDC) is not an option: the supported providers are
GitHub Actions, Google Cloud, ActiveState and GitLab CI/CD, and CircleCI support
is still open upstream ([pypi/warehouse#13888](https://github.com/pypi/warehouse/issues/13888)).
An API token is the authentication CircleCI has.

### What the job does, in order

1. `check_version.py` — the tag must name the declared version.
2. Requires the token for the index it will use.
3. Refuses to continue if that index already has this version (a re-run of an
   already-published version fails here, deliberately: it cannot be replaced,
   and `--skip-existing` would let a half-intended re-run look successful).
4. `python -m build` — wheel *and* sdist.
5. `twine check --strict dist/*` — the metadata a broken listing comes from.
6. Installs the built wheel into a clean virtualenv, imports it, and checks
   `opensysml.__version__` is the version being published.
7. `twine upload` with `TWINE_USERNAME=__token__` and the token from the
   context.

### Dry run on TestPyPI

A **pre-release version publishes to TestPyPI instead of PyPI** — that is the
whole rule, so the happy path has no extra switch to forget. To rehearse a
release:

```bash
# 1. Declare a pre-release version, e.g. VERSION = "0.3.0rc1"
$EDITOR python/opensysml/_version.py
# 2. Land it, then tag it
git tag -a opensysml-v0.3.0rc1 -m "opensysml 0.3.0rc1" && git push origin opensysml-v0.3.0rc1
```

The job resolves the version, sees a PEP 440 pre-release, requires
`TEST_PYPI_API_TOKEN`, and uploads to `https://test.pypi.org/legacy/`. Verify it
the same way as a real release, pointing pip at TestPyPI but taking the
dependencies from PyPI:

```bash
python -m venv /tmp/opensysml-rc && . /tmp/opensysml-rc/bin/activate
pip install --index-url https://test.pypi.org/simple/ \
            --extra-index-url https://pypi.org/simple/ opensysml==0.3.0rc1
python -c "import opensysml; print(opensysml.__version__)"
```

Then set `VERSION` to the final version and tag `opensysml-v0.3.0`.

Nothing about the pre-release path is required for a normal release; if you skip
it, no TestPyPI token is needed at all.

### Verifying an upload

In a clean virtualenv, from the index — not from the source tree:

```bash
python -m venv /tmp/opensysml-verify && . /tmp/opensysml-verify/bin/activate
pip install opensysml==0.3.0
python -c "import opensysml; print(opensysml.__version__)"    # must print 0.3.0
```

Then check the client end to end against a published core release, since that
is what a user gets:

```bash
export OPENSYSML_GRPC_VERSION=v0.0.5          # a released core tag
python -c "import opensysml; print(opensysml.load('examples/state-machine-demo.sysml').diagnostics)"
```

Finally, read the project page: the description, the license, the project URLs
and the Python versions are the metadata `twine check --strict` accepted, not
metadata anyone reviewed.

### If an upload goes wrong

A PyPI version cannot be replaced. Yank it
(PyPI → project → *Manage* → *Releases* → *Yank*, which hides it from resolvers
without breaking a pin that already names it), bump `VERSION`, and tag again.
Deleting a release frees nothing: the version number stays used.

## Releasing @opensysml/client to npm

The Node client in `clients/node/` is published to npm as `@opensysml/client` by
the `release-node` workflow, which runs on a tag matching `/^client-node-v.*/` —
for example `client-node-v0.1.0`. Nothing is published from a laptop, and a `v*`
core release tag publishes no package. **Nothing has been published yet**: the
first release needs the `@opensysml` scope to exist on npm, an automation token
for it, and the `npm` context below.

### Six packages, one tag

`@opensysml/client` carries no binary. The service binary comes from one of five
per-platform packages it names in `optionalDependencies`, which npm installs by
matching their `os`/`cpu` metadata:

| package | os | cpu |
| --- | --- | --- |
| `@opensysml/sysml-grpc-linux-x64` | linux | x64 |
| `@opensysml/sysml-grpc-linux-arm64` | linux | arm64 |
| `@opensysml/sysml-grpc-darwin-x64` | darwin | x64 |
| `@opensysml/sysml-grpc-darwin-arm64` | darwin | arm64 |
| `@opensysml/sysml-grpc-win32-x64` | win32 | x64 |

All six share the version in `clients/node/package.json`, because the
`optionalDependencies` name that exact version. The platform packages are
published first, so `@opensysml/client` is never on the registry naming a
version of them that is not. Where no package matches — a platform with no
release build — the client falls back to `$OPENSYSML_BINARY`, a binary in
`~/.opensysml/bin/`, `sysml-grpc` on `$PATH`, or an explicit external service;
it never downloads anything. See `clients/node/README.md`.

`build-node-binaries` cross-compiles the five binaries from the tagged revision
and writes a `.sha256` sidecar beside each; `npm run platform-packages` refuses
to package a binary whose bytes disagree with its sidecar, or that has none. The
bytes therefore never leave the pipeline that publishes them, which is the job
the Python client's pinned digests do for a download. npm's `--provenance` is not
used: the CLI mints attestations only on GitHub Actions and GitLab CI/CD.

### Why its own tag

The same reason as the Python client: the package resolves a service binary at
run time rather than being lockstep with a core release, so a client-only fix
should not wait for a core release, and a core release should not force an
immutable npm version. An npm version can be deprecated or (within 72 hours,
and only under conditions) unpublished, but never replaced — keeping that off
the re-runnable `v*` path is deliberate.

### What the job needs

1. The `@opensysml` **scope** on npm, with the publishing account a member of it.
2. An **automation** access token for that account (npm → *Access Tokens* →
   *Generate new token* → *Automation*, which bypasses 2FA for CI as a granular
   token restricted to the `@opensysml` scope).
3. A CircleCI **restricted context** named `npm` (Organization Settings →
   Contexts) holding it as `NPM_TOKEN`, restricted to a security group so only
   that group can run a job that reads it. A context reference is matched
   exactly, so the name is lower-case in both places.

### Releasing

```bash
# 1. Bump "version" in clients/node/package.json, land it.
# 2. Tag the version it declares.
git tag client-node-v0.1.0
git push origin client-node-v0.1.0
```

The job fails before publishing anything if the tag and
`clients/node/package.json` disagree, if any of the six versions is already on
the registry, if the Go suite or the client's own gate (`node-test`: build,
typecheck, lint, tests, conformance, and the mutation check that proves the
conformance runner is not vacuous) fails, or if a binary's digest does not match
its sidecar.

### If a publish goes wrong

An npm version cannot be replaced. `npm deprecate '@opensysml/client@0.1.0' '...'`
marks it, `npm unpublish` is possible only within 72 hours and only if nothing
depends on it, and either way the version number stays used: bump
`clients/node/package.json` and tag again.

## The final `pysysml` release

The client was published as [`pysysml`](https://pypi.org/project/pysysml/) up to
0.2.0, before the project was renamed. That name cannot be deleted and its last
version still installs and works, so `pip install pysysml` would otherwise go on
silently handing out a pre-rename client indefinitely.

`packaging/pypi-pysysml/` is the answer: `pysysml` 0.2.1, a distribution of the
same name whose only module raises `ImportError` naming `opensysml`. Being above
0.2.0 is what makes resolvers prefer it. It is not a compatibility shim — it does
not re-export `opensysml`, and it declares no dependency on it, since installing
the new client as a side effect would keep the old import working.

`pip install pysysml==0.2.0` is the escape hatch while migrating. An exact pin is
the only one that avoids the placeholder whatever version it carries: any range
that does not exclude it (`>=0.2`, `~=0.2.0`, `<1.0`) resolves to it, which is
the whole point.

It is released by its own tag, which runs the `release-pysysml-placeholder`
workflow:

```bash
git tag pysysml-v0.2.1    # must match the version in packaging/pypi-pysysml/pyproject.toml
git push origin pysysml-v0.2.1
```

The job resolves the version from the tag, refuses a version PyPI already has,
builds the wheel and sdist, and — the check that matters — installs the wheel
into a clean virtualenv and **fails if importing `pysysml` succeeds**. A
placeholder that imports cleanly is the alias this release exists not to be.
`python/tests/test_legacy_pysysml_placeholder.py` asserts the same contract from
source on every run.

This is expected to happen exactly once. Nothing further should be published
under the old name; a client fix goes to `opensysml`.
