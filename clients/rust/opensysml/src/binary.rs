//! Downloading, verifying and caching the `sysml-grpc` release binary.
//!
//! The cache at `~/.opensysml/bin` is shared with the Python client, metadata
//! shape included, so either client can tell which release it holds.
//!
//! # Known limitation: this client verifies pins only
//!
//! A download is verified against the digest table this crate ships
//! (`release-digests.json`, embedded at build time). Unlike the Python, Node and
//! Java clients, it does **not** verify the sigstore-signed `SHA256SUMS.txt`
//! manifest a release publishes, so a release this crate pins no digest for
//! cannot be verified here at all: it is refused. Setting
//! `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` is the only way through, and it accepts
//! the checksum served beside the binary, which is same-origin trust.

use std::borrow::Cow;
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::sync::{Mutex, OnceLock, PoisonError};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::error::Error;

/// Repository releases are downloaded from unless `$OPENSYSML_GITHUB_REPO` names another.
pub(crate) const DEFAULT_GITHUB_REPO: &str = "Open-MBEE/OpenSysML";

/// Origin serving release assets.
const DEFAULT_RELEASE_BASE_URL: &str = "https://github.com";

/// Origin answering the releases API.
const DEFAULT_API_BASE_URL: &str = "https://api.github.com";

/// Set to the repository whose unpinned downloads may be accepted (`1` for any).
const ALLOW_UNPINNED_ENV: &str = "OPENSYSML_ALLOW_UNPINNED_DOWNLOAD";

/// A cached binary is checked against the releases API before it is reused, so
/// every request here is bounded rather than left to hang.
const NETWORK_TIMEOUT: Duration = Duration::from_secs(15);

/// Release binaries are tens of megabytes; this only bounds a runaway response.
const MAX_DOWNLOAD_BYTES: u64 = 512 * 1024 * 1024;

/// The pinned digest table this crate ships, embedded so the published crate carries it.
const PINNED_DIGESTS: &str = include_str!("../release-digests.json");

/// Repository -> release tag -> asset name -> SHA-256 hex digest.
pub(crate) type PinTable = BTreeMap<String, BTreeMap<String, BTreeMap<String, String>>>;

/// The shipped pins, parsed once.
pub(crate) fn shipped_pins() -> &'static PinTable {
    static PINS: OnceLock<PinTable> = OnceLock::new();
    PINS.get_or_init(|| {
        serde_json::from_str(PINNED_DIGESTS)
            .expect("release-digests.json shipped with this crate is a digest table")
    })
}

/// What the cache metadata beside the binary records, shared with the Python client.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub(crate) struct CacheMetadata {
    /// Release tag downloaded, resolved (never `latest`).
    pub(crate) version: String,
    /// SHA-256 hex digest of the binary written.
    pub(crate) sha256: String,
    /// Repository (owner/repo) it was downloaded from.
    pub(crate) repo: String,
}

/// The `sysml-grpc` file name for the host platform.
pub(crate) fn binary_file_name() -> &'static str {
    if cfg!(windows) {
        "sysml-grpc.exe"
    } else {
        "sysml-grpc"
    }
}

/// The shared cache directory, `~/.opensysml/bin`.
pub(crate) fn default_cache_dir() -> PathBuf {
    #[cfg(windows)]
    let home = env::var_os("USERPROFILE").unwrap_or_default();
    #[cfg(not(windows))]
    let home = env::var_os("HOME").unwrap_or_default();
    PathBuf::from(home).join(".opensysml").join("bin")
}

/// Repository releases are downloaded from.
pub(crate) fn default_github_repo() -> String {
    env::var("OPENSYSML_GITHUB_REPO")
        .ok()
        .filter(|repo| !repo.trim().is_empty())
        .unwrap_or_else(|| DEFAULT_GITHUB_REPO.to_owned())
}

/// The release tag `$OPENSYSML_GRPC_VERSION` asks for, if any.
pub(crate) fn env_release_version() -> Option<String> {
    env::var("OPENSYSML_GRPC_VERSION")
        .ok()
        .map(|version| version.trim().to_owned())
        .filter(|version| !version.is_empty())
}

/// Name a release publishes the service binary for a platform under.
///
/// The names follow Go's `GOOS`/`GOARCH`, which is what the release pipeline builds for.
pub(crate) fn release_asset_name(os: &str, arch: &str) -> Result<String, Error> {
    let goos = match os {
        "linux" => "linux",
        "macos" => "darwin",
        "windows" => "windows",
        _ => {
            return Err(Error::UnsupportedPlatform {
                os: os.to_owned(),
                arch: arch.to_owned(),
            })
        }
    };
    let goarch = match arch {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        _ => {
            return Err(Error::UnsupportedPlatform {
                os: os.to_owned(),
                arch: arch.to_owned(),
            })
        }
    };
    // No windows/arm64 build is published, so naming one would only 404 later.
    if goos == "windows" && goarch == "arm64" {
        return Err(Error::UnsupportedPlatform {
            os: os.to_owned(),
            arch: arch.to_owned(),
        });
    }
    let name = format!("sysml-grpc-{goos}-{goarch}");
    Ok(if goos == "windows" {
        format!("{name}.exe")
    } else {
        name
    })
}

/// The release asset for the platform this crate was built for.
pub(crate) fn host_release_asset_name() -> Result<String, Error> {
    release_asset_name(env::consts::OS, env::consts::ARCH)
}

/// Whether an unpinned download from `repo` may fall back to same-origin trust.
///
/// `1` allows any repository; naming repositories keeps the trust with the fork
/// it is granted for.
fn unpinned_allowed(raw: Option<&str>, repo: &str) -> bool {
    let Some(value) = raw.map(str::trim).filter(|value| !value.is_empty()) else {
        return false;
    };
    match value.to_ascii_lowercase().as_str() {
        "0" | "false" | "no" => false,
        "1" | "true" | "yes" => true,
        _ => value.split(',').any(|named| named.trim() == repo),
    }
}

/// The SHA-256 hex digest of a file.
fn file_digest(path: &Path) -> Result<String, std::io::Error> {
    let mut file = fs::File::open(path)?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 64 * 1024];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    Ok(format!("{:x}", hasher.finalize()))
}

/// Downloads release binaries into the shared cache, verifying them against the shipped pins.
pub(crate) struct Downloader {
    repo: String,
    release_base_url: String,
    api_base_url: String,
    cache_dir: PathBuf,
    asset: String,
    allow_unpinned: Option<String>,
    pins: Cow<'static, PinTable>,
    agent: ureq::Agent,
    warnings: Mutex<Vec<String>>,
}

impl Downloader {
    /// A downloader configured from the environment, for the host platform.
    pub(crate) fn from_env() -> Result<Self, Error> {
        Ok(Self {
            repo: default_github_repo(),
            release_base_url: DEFAULT_RELEASE_BASE_URL.to_owned(),
            api_base_url: DEFAULT_API_BASE_URL.to_owned(),
            cache_dir: default_cache_dir(),
            asset: host_release_asset_name()?,
            allow_unpinned: env::var(ALLOW_UNPINNED_ENV).ok(),
            pins: Cow::Borrowed(shipped_pins()),
            agent: agent(),
            warnings: Mutex::new(Vec::new()),
        })
    }

    /// Path the cached binary is installed at.
    pub(crate) fn binary_path(&self) -> PathBuf {
        self.cache_dir.join(binary_file_name())
    }

    /// Path of the record of which release the cached binary was downloaded from.
    fn metadata_path(&self) -> PathBuf {
        let mut name = binary_file_name().to_owned();
        name.push_str(".json");
        self.cache_dir.join(name)
    }

    /// What was recorded beside the cached binary, or nothing readable was.
    fn read_metadata(&self) -> Option<CacheMetadata> {
        let recorded = fs::read_to_string(self.metadata_path()).ok()?;
        serde_json::from_str(&recorded).ok()
    }

    fn write_metadata(&self, metadata: &CacheMetadata) -> Result<(), Error> {
        let path = self.metadata_path();
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let recorded = serde_json::to_string(metadata).map_err(|error| {
            Error::BinaryDownload(format!("could not record the cache: {error}"))
        })?;
        fs::write(path, recorded)?;
        Ok(())
    }

    /// Warn the caller the way this client can: on stderr, and recorded for tests.
    fn warn(&self, message: String) {
        eprintln!("opensysml: warning: {message}");
        self.warnings
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .push(message);
    }

    /// The digest this crate pins for a release asset, if it pins one.
    fn pinned_digest(&self, version: &str, asset: &str) -> Option<&str> {
        self.pins
            .get(&self.repo)?
            .get(version)?
            .get(asset)
            .map(String::as_str)
    }

    /// The digest a download must have.
    ///
    /// The pin wins wherever there is one, and a served checksum contradicting
    /// it is tampering rather than a reason to prefer either. Where there is no
    /// pin this client has nothing left to verify against, because it does not
    /// check the release's signed checksum manifest.
    fn expected_digest(&self, version: &str, asset: &str, served: &str) -> Result<String, Error> {
        let Some(pinned) = self.pinned_digest(version, asset) else {
            if !unpinned_allowed(self.allow_unpinned.as_deref(), &self.repo) {
                return Err(Error::UnpinnedRelease(format!(
                    "opensysml pins no SHA-256 digest for {asset} of {version} of {self_repo}, \
                     and this Rust client does not verify the signed SHA256SUMS.txt manifest \
                     that the Python, Node and Java clients fall back to, so the only checksum \
                     available is the one served beside the binary, which a compromised release \
                     would serve too. Upgrade opensysml to a release that pins {version}, ask \
                     for a pinned release, or accept same-origin trust for this repository by \
                     setting ${ALLOW_UNPINNED_ENV}={self_repo} (or =1 for any repository).",
                    self_repo = self.repo,
                )));
            }
            self.warn(format!(
                "opensysml pins no digest for {asset} of {version} of {}; verifying it against \
                 the checksum served beside it, which detects corruption but not a compromised \
                 release (${ALLOW_UNPINNED_ENV} is set).",
                self.repo,
            ));
            return Ok(served.to_owned());
        };
        if served != pinned {
            return Err(Error::ChecksumMismatch(format!(
                "checksum mismatch for {asset} of {version}: {} serves {served}, but opensysml \
                 pins {pinned}. The release was republished with another binary, or the download \
                 is being tampered with; it was not installed.",
                self.repo,
            )));
        }
        Ok(pinned.to_owned())
    }

    fn download_url(&self, version: &str, asset: &str) -> String {
        format!(
            "{}/{}/releases/download/{version}/{asset}",
            self.release_base_url.trim_end_matches('/'),
            self.repo,
        )
    }

    fn fetch(&self, url: &str) -> Result<Vec<u8>, Error> {
        let response = self
            .agent
            .get(url)
            .call()
            .map_err(|error| Error::BinaryDownload(format!("{url}: {error}")))?;
        response
            .into_body()
            .with_config()
            .limit(MAX_DOWNLOAD_BYTES)
            .read_to_vec()
            .map_err(|error| Error::BinaryDownload(format!("{url}: {error}")))
    }

    /// Resolve the tag of the repository's latest published release.
    pub(crate) fn resolve_latest_version(&self) -> Result<String, Error> {
        let url = format!(
            "{}/repos/{}/releases/latest",
            self.api_base_url.trim_end_matches('/'),
            self.repo,
        );
        let body = self.fetch(&url)?;
        #[derive(Deserialize)]
        struct Release {
            tag_name: Option<String>,
        }
        let release: Release = serde_json::from_slice(&body).map_err(|error| {
            Error::BinaryDownload(format!(
                "failed to resolve latest release from {url}: {error}"
            ))
        })?;
        release
            .tag_name
            .filter(|tag| !tag.is_empty())
            .ok_or_else(|| {
                Error::BinaryDownload(format!("latest release of {} has no tag name", self.repo))
            })
    }

    /// Release tag the cached binary was downloaded from, if from this repository.
    ///
    /// The digest is re-checked, so a binary swapped in by hand is not read as
    /// the release it displaced, and forks publishing the same tags do not
    /// answer for each other.
    pub(crate) fn cached_release(&self) -> Option<String> {
        let recorded = self.read_metadata()?;
        if recorded.version.is_empty() || recorded.sha256.is_empty() || recorded.repo != self.repo {
            return None;
        }
        let actual = file_digest(&self.binary_path()).ok()?;
        (actual == recorded.sha256).then_some(recorded.version)
    }

    /// Why the cached binary is not the release that was asked for.
    ///
    /// Nothing asked for is answered by any cache, a locally built binary included.
    pub(crate) fn stale_cache_reason(&self, version: Option<&str>) -> Option<String> {
        let asked = match version {
            None => return None,
            Some("latest") => match self.resolve_latest_version() {
                Ok(resolved) => resolved,
                // Unreachable releases are no reason to discard a working cache.
                Err(_) => return None,
            },
            Some(version) => version.to_owned(),
        };
        let path = self.binary_path().display().to_string();
        match self.cached_release() {
            Some(have) if have == asked => None,
            Some(have) => Some(format!(
                "the binary cached at {path} is {have}, but {asked} was asked for"
            )),
            None => match self.read_metadata().map(|recorded| recorded.repo) {
                Some(recorded_repo) if recorded_repo != self.repo => Some(format!(
                    "the binary cached at {path} was downloaded from {recorded_repo}, but \
                     {asked} of {} was asked for",
                    self.repo,
                )),
                _ => Some(format!(
                    "the binary cached at {path} was not downloaded by this client, so which \
                     release it is cannot be told, and {asked} was asked for"
                )),
            },
        }
    }

    /// Download a release binary into the cache, verifying it before installing it.
    ///
    /// A failed or unverified download leaves any existing cached binary intact.
    pub(crate) fn download_binary(&self, version: &str) -> Result<PathBuf, Error> {
        let version = if version == "latest" {
            self.resolve_latest_version()?
        } else {
            version.to_owned()
        };
        let asset = self.asset.clone();
        let binary_path = self.binary_path();
        fs::create_dir_all(&self.cache_dir)?;

        let sidecar_url = self.download_url(&version, &format!("{asset}.sha256"));
        let sidecar = self.fetch(&sidecar_url)?;
        let served = String::from_utf8_lossy(&sidecar)
            .split_whitespace()
            .next()
            .map(str::to_owned)
            .ok_or_else(|| {
                Error::BinaryDownload(format!("{sidecar_url} served no checksum for {asset}"))
            })?;
        // Refusing an unverifiable release here means its binary is never fetched.
        let expected = self.expected_digest(&version, &asset, &served)?;

        let body = self.fetch(&self.download_url(&version, &asset))?;
        let temp_path = self.cache_dir.join(format!("{}.tmp", binary_file_name()));
        fs::write(&temp_path, &body)?;

        let actual = file_digest(&temp_path)?;
        if actual != expected {
            let _ = fs::remove_file(&temp_path);
            return Err(Error::ChecksumMismatch(format!(
                "checksum mismatch for {asset} of {version}: expected {expected}, but the \
                 download hashes to {actual}. It may be corrupted or tampered with; it was not \
                 installed."
            )));
        }

        if let Err(error) = fs::rename(&temp_path, &binary_path) {
            let _ = fs::remove_file(&temp_path);
            return Err(Error::BinaryDownload(format!(
                "downloaded {version} but could not install it at {}: {error}. A running service \
                 holding that file is the usual cause.",
                binary_path.display(),
            )));
        }

        // The cache is this user's, so no one else needs it.
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&binary_path, fs::Permissions::from_mode(0o700))?;
        }

        self.write_metadata(&CacheMetadata {
            version,
            sha256: expected,
            repo: self.repo.clone(),
        })?;
        Ok(binary_path)
    }

    /// The cached binary for the release asked for, downloading it when it is not already there.
    ///
    /// A cache that is the release asked for is reused; a stale one is replaced
    /// with a warning; a replacement that cannot be downloaded leaves the working
    /// cache in place, unless the download was refused for integrity reasons.
    pub(crate) fn ensure_binary(&self, version: Option<&str>) -> Result<PathBuf, Error> {
        let binary_path = self.binary_path();
        let mut cached = None;
        if binary_path.is_file() {
            match self.stale_cache_reason(version) {
                None => return Ok(binary_path),
                Some(stale) => {
                    let asked = version.unwrap_or("latest");
                    cached = Some(binary_path);
                    self.warn(format!(
                        "replacing the cached sysml-grpc: {stale}. Downloading {asked}."
                    ));
                }
            }
        }
        let Some(version) = version else {
            return Err(Error::BinaryDownload(format!(
                "no sysml-grpc binary is cached at {} and no release was asked for; build one, \
                 or set $OPENSYSML_GRPC_VERSION to download one.",
                self.binary_path().display(),
            )));
        };

        match self.download_binary(version) {
            Ok(path) => Ok(path),
            // A download that may have been tampered with is never answered from the cache.
            Err(error @ Error::ChecksumMismatch(_)) => Err(error),
            Err(error) => match cached {
                // A release that cannot be had is no reason to lose a working binary.
                Some(path) => {
                    self.warn(format!(
                        "keeping the cached sysml-grpc at {}: {version} was not downloaded \
                         ({error}). It may be an older release than asked for.",
                        path.display(),
                    ));
                    Ok(path)
                }
                None => Err(error),
            },
        }
    }

    #[cfg(test)]
    fn warnings(&self) -> Vec<String> {
        self.warnings
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .clone()
    }
}

fn agent() -> ureq::Agent {
    ureq::Agent::config_builder()
        .timeout_global(Some(NETWORK_TIMEOUT))
        .build()
        .into()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;
    use std::io::Write;
    use std::net::{TcpListener, TcpStream};
    use std::sync::Arc;
    use std::thread;

    /// A release origin serving fixture bytes, so no test touches the network.
    struct FixtureServer {
        base_url: String,
    }

    impl FixtureServer {
        fn start(routes: HashMap<String, Vec<u8>>) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").expect("bind a fixture port");
            let base_url = format!("http://{}", listener.local_addr().expect("local address"));
            let routes = Arc::new(routes);
            thread::spawn(move || {
                for stream in listener.incoming() {
                    let Ok(stream) = stream else { break };
                    let routes = Arc::clone(&routes);
                    thread::spawn(move || serve(stream, &routes));
                }
            });
            Self { base_url }
        }
    }

    fn serve(mut stream: TcpStream, routes: &HashMap<String, Vec<u8>>) {
        let mut request = [0u8; 4096];
        let read = match stream.read(&mut request) {
            Ok(read) if read > 0 => read,
            _ => return,
        };
        let head = String::from_utf8_lossy(&request[..read]).to_string();
        let path = head
            .split_whitespace()
            .nth(1)
            .unwrap_or_default()
            .to_owned();
        let response = match routes.get(&path) {
            Some(body) => {
                let mut response = format!(
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                )
                .into_bytes();
                response.extend_from_slice(body);
                response
            }
            None => {
                b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_vec()
            }
        };
        let _ = stream.write_all(&response);
        let _ = stream.flush();
    }

    struct Harness {
        downloader: Downloader,
        _cache: tempdir::TempDir,
    }

    /// A minimal owned temporary directory, so the tests add no dependency.
    mod tempdir {
        use std::fs;
        use std::path::{Path, PathBuf};
        use std::sync::atomic::{AtomicU64, Ordering};

        pub struct TempDir(PathBuf);

        impl TempDir {
            pub fn new(label: &str) -> Self {
                static COUNTER: AtomicU64 = AtomicU64::new(0);
                let unique = format!(
                    "opensysml-{label}-{}-{}",
                    std::process::id(),
                    COUNTER.fetch_add(1, Ordering::Relaxed)
                );
                let path = std::env::temp_dir().join(unique);
                fs::create_dir_all(&path).expect("create a temporary directory");
                Self(path)
            }

            pub fn path(&self) -> &Path {
                &self.0
            }
        }

        impl Drop for TempDir {
            fn drop(&mut self) {
                let _ = fs::remove_dir_all(&self.0);
            }
        }
    }

    const ASSET: &str = "sysml-grpc-linux-amd64";
    const REPO: &str = "Open-MBEE/OpenSysML";

    fn digest_of(bytes: &[u8]) -> String {
        format!("{:x}", Sha256::digest(bytes))
    }

    fn release_routes(version: &str, body: &[u8], served_digest: &str) -> HashMap<String, Vec<u8>> {
        let mut routes = HashMap::new();
        routes.insert(
            format!("/{REPO}/releases/download/{version}/{ASSET}"),
            body.to_vec(),
        );
        routes.insert(
            format!("/{REPO}/releases/download/{version}/{ASSET}.sha256"),
            format!("{served_digest}  {ASSET}\n").into_bytes(),
        );
        routes.insert(
            format!("/repos/{REPO}/releases/latest"),
            format!(r#"{{"tag_name": "{version}"}}"#).into_bytes(),
        );
        routes
    }

    fn harness(label: &str, routes: HashMap<String, Vec<u8>>, pins: PinTable) -> Harness {
        let server = FixtureServer::start(routes);
        let cache = tempdir::TempDir::new(label);
        Harness {
            downloader: Downloader {
                repo: REPO.to_owned(),
                release_base_url: server.base_url.clone(),
                api_base_url: server.base_url,
                cache_dir: cache.path().to_path_buf(),
                asset: ASSET.to_owned(),
                allow_unpinned: None,
                pins: Cow::Owned(pins),
                agent: agent(),
                warnings: Mutex::new(Vec::new()),
            },
            _cache: cache,
        }
    }

    fn pin_table(version: &str, digest: &str) -> PinTable {
        let mut assets = BTreeMap::new();
        assets.insert(ASSET.to_owned(), digest.to_owned());
        let mut releases = BTreeMap::new();
        releases.insert(version.to_owned(), assets);
        let mut table = PinTable::new();
        table.insert(REPO.to_owned(), releases);
        table
    }

    #[test]
    fn a_pinned_release_is_downloaded_verified_and_cached() {
        let body = b"the pinned release binary";
        let digest = digest_of(body);
        let harness = harness(
            "pinned",
            release_routes("v0.1.0", body, &digest),
            pin_table("v0.1.0", &digest),
        );
        let downloader = &harness.downloader;

        let path = downloader.download_binary("v0.1.0").expect("download");
        assert_eq!(fs::read(&path).expect("read the cache"), body);
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.1.0"));
        assert!(!downloader.cache_dir.join("sysml-grpc.tmp").exists());
        assert!(downloader.warnings().is_empty());

        let recorded = downloader.read_metadata().expect("metadata");
        assert_eq!(recorded.version, "v0.1.0");
        assert_eq!(recorded.sha256, digest);
        assert_eq!(recorded.repo, REPO);

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = fs::metadata(&path).expect("stat").permissions().mode();
            assert_eq!(mode & 0o777, 0o700);
        }
    }

    #[test]
    fn latest_is_resolved_through_the_releases_api() {
        let body = b"the latest release binary";
        let digest = digest_of(body);
        let harness = harness(
            "latest",
            release_routes("v0.2.0", body, &digest),
            pin_table("v0.2.0", &digest),
        );

        let path = harness
            .downloader
            .download_binary("latest")
            .expect("download");
        assert_eq!(fs::read(path).expect("read the cache"), body);
        assert_eq!(
            harness.downloader.cached_release().as_deref(),
            Some("v0.2.0")
        );
    }

    #[test]
    fn a_served_checksum_contradicting_the_pin_is_refused() {
        let body = b"the release the origin serves";
        let harness = harness(
            "contradicted",
            release_routes("v0.1.0", body, &digest_of(body)),
            pin_table("v0.1.0", &"cd".repeat(32)),
        );
        let downloader = &harness.downloader;
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(b"the cached binary"),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");
        fs::write(downloader.binary_path(), b"the cached binary").expect("place a cache");

        let error = downloader.download_binary("v0.1.0").expect_err("refusal");
        assert!(
            matches!(&error, Error::ChecksumMismatch(message) if message.contains("opensysml pins")),
            "{error}"
        );
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            b"the cached binary"
        );
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.0.5"));
        assert!(!downloader.cache_dir.join("sysml-grpc.tmp").exists());
    }

    #[test]
    fn a_tampered_download_is_refused_and_leaves_the_cache_alone() {
        // Pin and sidecar agree; the body served is not the binary they name.
        let promised = digest_of(b"the release that was pinned");
        let harness = harness(
            "tampered",
            release_routes("v0.1.0", b"truncated", &promised),
            pin_table("v0.1.0", &promised),
        );
        let downloader = &harness.downloader;
        fs::write(downloader.binary_path(), b"the cached binary").expect("place a cache");

        let error = downloader.download_binary("v0.1.0").expect_err("refusal");
        assert!(
            matches!(&error, Error::ChecksumMismatch(message) if message.contains("hashes to")),
            "{error}"
        );
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            b"the cached binary"
        );
        assert!(!downloader.cache_dir.join("sysml-grpc.tmp").exists());
    }

    #[test]
    fn an_unpinned_release_is_refused_naming_the_gap() {
        let body = b"an unpinned release binary";
        let harness = harness(
            "unpinned",
            release_routes("v9.9.9", body, &digest_of(body)),
            PinTable::new(),
        );

        let error = harness
            .downloader
            .download_binary("v9.9.9")
            .expect_err("refusal");
        let Error::UnpinnedRelease(message) = &error else {
            panic!("expected an unpinned refusal, got {error}");
        };
        assert!(message.contains("pins no SHA-256 digest"), "{message}");
        assert!(message.contains("v9.9.9"), "{message}");
        assert!(
            message.contains("does not verify the signed SHA256SUMS.txt manifest"),
            "{message}"
        );
        assert!(message.contains(ALLOW_UNPINNED_ENV), "{message}");
        assert!(!harness.downloader.binary_path().exists());
    }

    #[test]
    fn an_unpinned_release_is_accepted_with_a_warning_when_opted_in() {
        for opt_in in ["1", REPO] {
            let body = b"an unpinned release binary";
            let digest = digest_of(body);
            let mut harness = harness(
                "unpinned-allowed",
                release_routes("v9.9.9", body, &digest),
                PinTable::new(),
            );
            harness.downloader.allow_unpinned = Some(opt_in.to_owned());

            let path = harness
                .downloader
                .download_binary("v9.9.9")
                .expect("download");
            assert_eq!(fs::read(path).expect("read the cache"), body);
            let warnings = harness.downloader.warnings();
            assert!(
                warnings
                    .iter()
                    .any(|warning| warning.contains("pins no digest")
                        && warning.contains("not a compromised release")),
                "{warnings:?}"
            );
        }
    }

    #[test]
    fn opting_in_for_one_repository_is_not_opting_in_for_another() {
        assert!(!unpinned_allowed(Some("a-fork/OpenSysML"), REPO));
        assert!(unpinned_allowed(
            Some("a-fork/OpenSysML"),
            "a-fork/OpenSysML"
        ));
        assert!(unpinned_allowed(
            Some("a-fork/OpenSysML, Open-MBEE/OpenSysML"),
            REPO
        ));
        assert!(unpinned_allowed(Some("1"), REPO));
        assert!(!unpinned_allowed(Some("0"), REPO));
        assert!(!unpinned_allowed(Some(""), REPO));
        assert!(!unpinned_allowed(None, REPO));
    }

    #[test]
    fn a_stale_cache_is_replaced_with_a_warning() {
        let body = b"the release asked for";
        let digest = digest_of(body);
        let harness = harness(
            "stale",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        let older = b"the release before";
        fs::write(downloader.binary_path(), older).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(older),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        let reason = downloader
            .stale_cache_reason(Some("v0.0.7"))
            .expect("a stale cache");
        assert!(
            reason.contains("v0.0.5") && reason.contains("v0.0.7"),
            "{reason}"
        );

        let path = downloader
            .ensure_binary(Some("v0.0.7"))
            .expect("replacement");
        assert_eq!(fs::read(path).expect("read the cache"), body);
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.0.7"));
        let warnings = downloader.warnings();
        assert!(
            warnings
                .iter()
                .any(|warning| warning.contains("replacing the cached sysml-grpc")),
            "{warnings:?}"
        );
    }

    #[test]
    fn a_cache_survives_a_replacement_that_cannot_be_downloaded() {
        // The fixture origin publishes v0.0.7 only, so v0.0.9 is a 404.
        let body = b"the release published";
        let digest = digest_of(body);
        let harness = harness(
            "kept",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        let cached = b"the working cache";
        fs::write(downloader.binary_path(), cached).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(cached),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        let path = downloader.ensure_binary(Some("v0.0.9")).expect("the cache");
        assert_eq!(fs::read(path).expect("read the cache"), cached);
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.0.5"));
        let warnings = downloader.warnings();
        assert!(
            warnings
                .iter()
                .any(|warning| warning.contains("keeping the cached sysml-grpc")),
            "{warnings:?}"
        );
    }

    #[test]
    fn a_cache_holding_the_release_asked_for_is_reused() {
        let body = b"the release asked for";
        let digest = digest_of(body);
        let harness = harness(
            "reused",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        fs::write(downloader.binary_path(), body).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest.clone(),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        assert!(downloader.stale_cache_reason(Some("v0.0.7")).is_none());
        assert!(downloader.stale_cache_reason(None).is_none());
        assert_eq!(
            downloader.ensure_binary(Some("v0.0.7")).expect("the cache"),
            downloader.binary_path()
        );
        assert!(downloader.warnings().is_empty());
    }

    #[test]
    fn a_binary_swapped_under_a_recorded_name_is_not_that_release() {
        let harness = harness("swapped", HashMap::new(), PinTable::new());
        let downloader = &harness.downloader;
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest_of(b"downloaded"),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");
        fs::write(downloader.binary_path(), b"something else entirely").expect("swap it");

        assert!(downloader.cached_release().is_none());
        assert!(downloader
            .stale_cache_reason(Some("v0.0.7"))
            .expect("stale")
            .contains("cannot be told"));
    }

    #[test]
    fn a_cache_from_another_repository_does_not_answer_for_this_one() {
        let mut harness = harness("fork", HashMap::new(), PinTable::new());
        harness.downloader.repo = "someone/fork".to_owned();
        let downloader = &harness.downloader;
        let body = b"the other repository's build";
        fs::write(downloader.binary_path(), body).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest_of(body),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        assert!(downloader.cached_release().is_none());
        let reason = downloader
            .stale_cache_reason(Some("v0.0.7"))
            .expect("stale");
        assert!(
            reason.contains("downloaded from Open-MBEE/OpenSysML"),
            "{reason}"
        );
        assert!(reason.contains("someone/fork"), "{reason}");
    }

    #[test]
    fn an_unreachable_release_query_keeps_a_working_cache() {
        // The fixture origin answers nothing, so resolving 'latest' fails.
        let harness = harness("offline", HashMap::new(), PinTable::new());
        let downloader = &harness.downloader;
        let body = b"the working cache";
        fs::write(downloader.binary_path(), body).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest_of(body),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        assert!(downloader.stale_cache_reason(Some("latest")).is_none());
        assert_eq!(
            downloader.ensure_binary(Some("latest")).expect("the cache"),
            downloader.binary_path()
        );
    }

    #[test]
    fn no_cache_and_no_release_asked_for_is_an_error() {
        let harness = harness("nothing", HashMap::new(), PinTable::new());
        let error = harness.downloader.ensure_binary(None).expect_err("refusal");
        assert!(
            matches!(&error, Error::BinaryDownload(message)
                if message.contains("OPENSYSML_GRPC_VERSION")),
            "{error}"
        );
    }

    #[test]
    fn release_assets_are_named_for_the_platform() {
        assert_eq!(
            release_asset_name("linux", "x86_64").expect("linux/amd64"),
            "sysml-grpc-linux-amd64"
        );
        assert_eq!(
            release_asset_name("linux", "aarch64").expect("linux/arm64"),
            "sysml-grpc-linux-arm64"
        );
        assert_eq!(
            release_asset_name("macos", "x86_64").expect("darwin/amd64"),
            "sysml-grpc-darwin-amd64"
        );
        assert_eq!(
            release_asset_name("macos", "aarch64").expect("darwin/arm64"),
            "sysml-grpc-darwin-arm64"
        );
        assert_eq!(
            release_asset_name("windows", "x86_64").expect("windows/amd64"),
            "sysml-grpc-windows-amd64.exe"
        );
        for (os, arch) in [
            ("windows", "aarch64"),
            ("freebsd", "x86_64"),
            ("linux", "riscv64"),
        ] {
            let error = release_asset_name(os, arch).expect_err("unsupported");
            assert!(
                matches!(&error, Error::UnsupportedPlatform { os: reported, .. } if reported == os),
                "{error}"
            );
        }
    }

    #[test]
    fn every_shipped_pin_is_a_sha256_of_a_published_asset() {
        let published: Vec<String> = [
            ("linux", "x86_64"),
            ("linux", "aarch64"),
            ("macos", "x86_64"),
            ("macos", "aarch64"),
            ("windows", "x86_64"),
        ]
        .iter()
        .map(|(os, arch)| release_asset_name(os, arch).expect("a published platform"))
        .collect();

        let pins = shipped_pins();
        assert!(
            !pins.is_empty(),
            "no release is pinned, so every download is refused"
        );
        for (repo, releases) in pins {
            assert!(repo.contains('/'), "{repo}");
            for (version, assets) in releases {
                assert!(version.starts_with('v'), "{repo} {version}");
                let names: Vec<&String> = assets.keys().collect();
                assert_eq!(
                    names.len(),
                    published.len(),
                    "{repo} {version} does not pin every published platform"
                );
                for asset in &published {
                    let digest = assets
                        .get(asset)
                        .unwrap_or_else(|| panic!("{repo} {version} pins no {asset}"));
                    assert!(
                        digest.len() == 64
                            && digest
                                .chars()
                                .all(|c| c.is_ascii_hexdigit() && !c.is_ascii_uppercase()),
                        "{repo} {version} {asset}: {digest}"
                    );
                }
            }
        }
    }
}
