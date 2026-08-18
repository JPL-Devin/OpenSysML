# Releasing Systemica

A release is cut by pushing a `v*` tag. Everything after that is CircleCI: the
`release` workflow runs the test suite, cross-compiles `sysml`, `sysml-lsp` and
`sysml-grpc` for five platforms, and publishes them to a GitHub release. Nothing
is published from a laptop.

The Python client is released separately, by a `pysysml-v*` tag, which runs the
`release-python` workflow and uploads `pysysml` to PyPI — see
[Releasing pysysml to PyPI](#releasing-pysysml-to-pypi). The two are independent
on purpose: `pysysml` resolves a `sysml-grpc` binary at runtime from whichever
release the caller names, not from a release matching its own version.

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
`pysysml-v*` release gates on the same suite:

```bash
make build-grpc && mkdir -p ~/.pysysml/bin && cp bin/sysml-grpc ~/.pysysml/bin/
pip install -e python/ && pip install pytest pytest-mock
pytest python/tests/ -v
```

The OMG training-corpus gate skips while the corpus is absent, so fetch it and
run it explicitly — the expected result is the pinned baseline, currently
98/100 files clean:

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
`JPL-Devin/Systemica`, which has no tags at all — so cutting a release means
promoting `main` upstream first (v0.0.4 came through Open-MBEE PR #47) and
tagging there. Tagging the development repository would build a release nobody
consumes.

Tags are matched by `/^v.*/` in `.circleci/config.yml`. A tag on a commit that
fails the suite fails the release workflow before anything is published.

## What CircleCI publishes

`build-release` produces, in `dist/`:

- per-binary archives — `sysml-<os>-<arch>.tar.gz`,
  `sysml-lsp-<os>-<arch>.tar.gz` (`.zip` on Windows);
- bundle archives — `systemica-<os>-<arch>.tar.gz` holding both binaries under
  their plain names, which is the layout Homebrew and a PATH install expect;
- `sysml-grpc-<os>-<arch>`, published raw with a `.sha256` sidecar rather than
  archived, because that is what `pysysml` downloads and verifies
  (`python/pysysml/binary.py`) when it starts the service for a Python caller;
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
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/systemica-linux-amd64.tar.gz
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/SHA256SUMS.txt
   sha256sum -c SHA256SUMS.txt --ignore-missing
   tar xzf systemica-linux-amd64.tar.gz && ./sysml --version
   ```

   `--version` must report the tag, not `dev`.

   Then check the path `pysysml` takes, since it reads the sidecar rather than
   `SHA256SUMS.txt`:

   ```bash
   PYSYSML_GITHUB_REPO=Open-MBEE/OpenSysML python -c \
     "from pysysml.binary import download_binary; print(download_binary('latest'))"
   ~/.pysysml/bin/sysml-grpc -version
   ```

   A checksum mismatch there means the sidecar and the binary came from
   different builds.

2. **Let the Homebrew tap pick the release up.** The tap repository
   `Open-MBEE/homebrew-tap` updates itself: a scheduled workflow there resolves
   the latest `Open-MBEE/OpenSysML` release, renders `Formula/systemica.rb` from
   this repository's `scripts/render-homebrew-formula.sh` and formula template at
   that tag, and commits only when the file changed. Nothing here triggers it, so
   the formula follows the release within the workflow's schedule interval.

   If it does not, check the workflow run in the tap repository. The render reads
   the release's `SHA256SUMS.txt`, so a release missing that asset (or missing a
   `systemica-<os>-<arch>.tar.gz` line in it) fails the run loudly instead of
   committing a broken formula — re-run `publish-github-release` for the tag and
   then the tap workflow (`workflow_dispatch`). Rendering by hand still works:

   ```bash
   scripts/render-homebrew-formula.sh v0.0.5 > Formula/systemica.rb
   ```

   See [packaging/homebrew/README.md](../../packaging/homebrew/README.md).

3. **Say what is not signed.** macOS binaries are not Developer ID signed or
   notarized and Windows binaries are not Authenticode signed, so a browser
   download trips Gatekeeper or SmartScreen. Point release notes at
   [MACOS_DISTRIBUTION.md](macos-distribution.md), which gives the workarounds
   and what signing would take.

### Pinned release digests

`pysysml` refuses to run a `sysml-grpc` binary it has no digest for, so a new
core release has to be pinned into `PINNED_SHA256` in `python/pysysml/binary.py`
before a client release can resolve it:

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

## Releasing pysysml to PyPI

The Python client in `python/` is published to PyPI as
[`pysysml`](https://pypi.org/project/pysysml/) by the `release-python` workflow,
which runs on a tag matching `/^pysysml-v.*/` — for example `pysysml-v0.1.0`.
Nothing is uploaded from a laptop, and a `v*` core release tag publishes no
package.

### Why its own tag

`pysysml` does not ship the service: it downloads a `sysml-grpc` binary at
runtime for whatever release the caller names (`version=`,
`$PYSYSML_GRPC_VERSION`, or `latest`), verifying it against the digest it pins
for that release (`PINNED_SHA256` in `python/pysysml/binary.py`, regenerated by
`python/scripts/pin_release_checksums.py` after a core release publishes its
assets). Its version therefore says nothing about which core release it runs
against, and tying the two together would put a new, immutable PyPI version on
every core release and would block a client-only fix behind a core release.

Keeping them apart also protects the `v*` path: `publish-github-release` runs
`ghr -replace`, so re-running a core release is an ordinary operation, while a
PyPI version can be yanked but never re-uploaded. A re-run must never have an
irreversible upload hanging off it.

### The version, in one place

`python/pysysml/_version.py` is the only declaration:

- `python/pyproject.toml` has `dynamic = ["version"]` and reads
  `pysysml._version.VERSION` (there is no `setup.py` any more —
  `pyproject.toml` declares the build);
- `pysysml.__version__` reports that declaration, which ships beside the module
  and is therefore the version of the code being imported. A wheel's metadata is
  generated from it, so the two agree there; an editable install's dist-info is
  written once, at install time, and a checkout that bumps `VERSION` afterwards
  would otherwise report the version it had when `pip install -e` ran.

`python/tests/test_version.py` fails if a second version literal reappears
anywhere under `python/`, or if the declaration, the installed metadata and
`__version__` stop agreeing. Where the install is editable, the tests locate the
package through the install's own PEP 610 record (`pysysml/_dist.py`) rather than
the dist-info's directory, which for an editable install is a site-packages path
holding no `pysysml/` at all.

The tag must name the declared version. `python/scripts/check_version.py` is run
by the job before anything is built, and fails loudly otherwise:

```bash
python python/scripts/check_version.py --tag pysysml-v0.1.0   # prints 0.1.0
```

So a release is: bump `VERSION` in `python/pysysml/_version.py`, land it, then
tag `pysysml-v<that version>`.

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

### First upload versus later ones

`pysysml` does not exist on PyPI yet
(`https://pypi.org/pypi/pysysml/json` → 404), and a project-scoped token cannot
be created for a project that does not exist. So:

1. **Before the first release**, create an **account-scoped** API token
   (PyPI → Account settings → API tokens → *Add API token*, scope *Entire
   account*) and put it in the `PyPI` context as `PYPI_API_TOKEN`. Treat
   it as a credential that can publish anything the account owns.
2. **Immediately after the first upload succeeds**, replace it: create a token
   scoped to the `pysysml` project only, update `PYPI_API_TOKEN` in the context,
   and **revoke the account-scoped token**. This step is part of the first
   release, not a follow-up — leaving an account-scoped token in CI is the
   avoidable risk here.
3. Add a second owner/maintainer to the PyPI project at the same time, so the
   project is not tied to one account.

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
   `pysysml.__version__` is the version being published.
7. `twine upload` with `TWINE_USERNAME=__token__` and the token from the
   context.

### Dry run on TestPyPI

A **pre-release version publishes to TestPyPI instead of PyPI** — that is the
whole rule, so the happy path has no extra switch to forget. To rehearse a
release:

```bash
# 1. Declare a pre-release version, e.g. VERSION = "0.1.0rc1"
$EDITOR python/pysysml/_version.py
# 2. Land it, then tag it
git tag -a pysysml-v0.1.0rc1 -m "pysysml 0.1.0rc1" && git push origin pysysml-v0.1.0rc1
```

The job resolves the version, sees a PEP 440 pre-release, requires
`TEST_PYPI_API_TOKEN`, and uploads to `https://test.pypi.org/legacy/`. Verify it
the same way as a real release, pointing pip at TestPyPI but taking the
dependencies from PyPI:

```bash
python -m venv /tmp/pysysml-rc && . /tmp/pysysml-rc/bin/activate
pip install --index-url https://test.pypi.org/simple/ \
            --extra-index-url https://pypi.org/simple/ pysysml==0.1.0rc1
python -c "import pysysml; print(pysysml.__version__)"
```

Then set `VERSION` to the final version and tag `pysysml-v0.1.0`.

Nothing about the pre-release path is required for a normal release; if you skip
it, no TestPyPI token is needed at all.

### Verifying an upload

In a clean virtualenv, from the index — not from the source tree:

```bash
python -m venv /tmp/pysysml-verify && . /tmp/pysysml-verify/bin/activate
pip install pysysml==0.1.0
python -c "import pysysml; print(pysysml.__version__)"    # must print 0.1.0
```

Then check the client end to end against a published core release, since that
is what a user gets:

```bash
export PYSYSML_GRPC_VERSION=v0.0.5          # a released core tag
python -c "import pysysml; print(pysysml.load('examples/state-machine-demo.sysml').diagnostics)"
```

Finally, read the project page: the description, the license, the project URLs
and the Python versions are the metadata `twine check --strict` accepted, not
metadata anyone reviewed.

### If an upload goes wrong

A PyPI version cannot be replaced. Yank it
(PyPI → project → *Manage* → *Releases* → *Yank*, which hides it from resolvers
without breaking a pin that already names it), bump `VERSION`, and tag again.
Deleting a release frees nothing: the version number stays used.
