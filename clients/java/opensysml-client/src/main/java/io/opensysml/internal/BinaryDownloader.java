package io.opensysml.internal;

import io.opensysml.ChecksumMismatchException;
import io.opensysml.ConnectionOptions;
import io.opensysml.ServiceStartException;
import io.opensysml.UnpinnedReleaseException;
import io.opensysml.UnsignedReleaseException;
import io.opensysml.internal.SignedManifest.ReleaseSigner;
import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.nio.file.attribute.PosixFilePermission;
import java.nio.file.attribute.PosixFilePermissions;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Duration;
import java.util.HexFormat;
import java.util.Locale;
import java.util.Optional;
import java.util.Set;
import java.util.function.Consumer;
import java.util.function.UnaryOperator;

/**
 * Downloads a {@code sysml-grpc} release binary into the cache the Python client also reads.
 *
 * <p>What a download is verified against is ordered, and each step is a refusal rather than a
 * fallback: the digest this client ships a pin for wins, a release nothing is pinned for is
 * vouched for by the signature on its {@code SHA256SUMS.txt}, and a release with neither is refused
 * unless same-origin trust is granted explicitly with {@code $OPENSYSML_ALLOW_UNPINNED_DOWNLOAD}.
 * The binary is written to a temporary file, verified, and only then moved over the cache, so a
 * failed or unverified download leaves a working cached binary alone.
 */
public final class BinaryDownloader {

  /** Repository releases are downloaded from unless another one is named. */
  public static final String DEFAULT_GITHUB_REPO = "Open-MBEE/OpenSysML";

  /**
   * A cached binary is checked against the releases API before being reused, and that happens
   * while the service-start lock is held, so it must not hang there.
   */
  static final Duration NETWORK_TIMEOUT = Duration.ofSeconds(15);

  private static final System.Logger LOG =
      System.getLogger(BinaryDownloader.class.getName());

  private final String downloadBaseUrl;
  private final String apiBaseUrl;
  private final Path binaryPath;
  private final ReleaseDigests pins;
  private final Path trustedRoot;
  private final String asset;
  private final UnaryOperator<String> environment;
  private final Consumer<String> warnings;
  private final HttpClient http;

  private BinaryDownloader(Builder builder) {
    this.downloadBaseUrl = builder.downloadBaseUrl;
    this.apiBaseUrl = builder.apiBaseUrl;
    this.binaryPath = builder.binaryPath;
    this.pins = builder.pins;
    this.trustedRoot = builder.trustedRoot;
    this.asset = builder.asset;
    this.environment = builder.environment;
    this.warnings = builder.warnings;
    this.http =
        HttpClient.newBuilder()
            .connectTimeout(NETWORK_TIMEOUT)
            .followRedirects(HttpClient.Redirect.NORMAL)
            .build();
  }

  /**
   * A downloader reading the releases of the real GitHub, with the pins this jar ships.
   *
   * @return the downloader a connection uses
   */
  public static BinaryDownloader production() {
    return new Builder().build();
  }

  /**
   * Repository releases are downloaded from, from the options, the environment, or the default.
   *
   * @param options how the connection was configured
   * @return owner/repo
   */
  public static String githubRepo(ConnectionOptions options) {
    return githubRepo(options, System::getenv);
  }

  static String githubRepo(ConnectionOptions options, UnaryOperator<String> environment) {
    Optional<String> named = options.githubRepo();
    if (named.isPresent()) {
      return named.get();
    }
    String set = environment.apply(ConnectionOptions.REPO_ENV);
    return set == null || set.isBlank() ? DEFAULT_GITHUB_REPO : set.trim();
  }

  /**
   * The release a caller asked to be downloaded, from the options or the environment.
   *
   * @param options how the connection was configured
   * @return a release tag, {@code latest}, or empty when no download was asked for
   */
  public static Optional<String> versionAskedFor(ConnectionOptions options) {
    return versionAskedFor(options, System::getenv);
  }

  static Optional<String> versionAskedFor(
      ConnectionOptions options, UnaryOperator<String> environment) {
    if (options.downloadVersion().isPresent()) {
      return options.downloadVersion();
    }
    String set = environment.apply(ConnectionOptions.VERSION_ENV);
    return set == null || set.isBlank() ? Optional.empty() : Optional.of(set.trim());
  }

  /**
   * Whether an unpinned download from a repository may fall back to same-origin trust.
   *
   * @param options how the connection was configured
   * @param githubRepo repository (owner/repo) being downloaded from
   * @return {@code true} when the option is set, or the variable is {@code 1} or names this
   *     repository
   */
  public static boolean unpinnedDownloadsAllowed(ConnectionOptions options, String githubRepo) {
    return unpinnedDownloadsAllowed(options, githubRepo, System::getenv);
  }

  static boolean unpinnedDownloadsAllowed(
      ConnectionOptions options, String githubRepo, UnaryOperator<String> environment) {
    if (options.allowUnpinnedDownload()) {
      return true;
    }
    String set = environment.apply(ConnectionOptions.ALLOW_UNPINNED_ENV);
    String allowed = set == null ? "" : set.trim();
    String lowered = allowed.toLowerCase(Locale.ROOT);
    if (lowered.isEmpty() || lowered.equals("0") || lowered.equals("false")
        || lowered.equals("no")) {
      return false;
    }
    if (lowered.equals("1") || lowered.equals("true") || lowered.equals("yes")) {
      return true;
    }
    // Naming repositories keeps the trust with the fork it is granted for.
    for (String named : allowed.split(",")) {
      if (named.trim().equals(githubRepo)) {
        return true;
      }
    }
    return false;
  }

  /**
   * Where a downloaded binary is cached.
   *
   * @return the cached binary's path, which need not exist
   */
  public Path binaryPath() {
    return binaryPath;
  }

  /**
   * Where the release the cached binary came from is recorded, shared with the Python client.
   *
   * @return the metadata sidecar's path
   */
  public Path metadataPath() {
    return binaryPath.resolveSibling(binaryPath.getFileName() + ".json");
  }

  /**
   * The digest this client pins for a release asset.
   *
   * @param version release tag, resolved (never {@code latest})
   * @param asset asset name
   * @param githubRepo repository (owner/repo)
   * @return the hex digest, or empty when nothing is pinned for it
   */
  public Optional<String> pinnedDigest(String version, String asset, String githubRepo) {
    return pins.pin(githubRepo, version, asset);
  }

  /**
   * The URL a release publishes an asset at.
   *
   * @param version release tag, resolved (never {@code latest})
   * @param asset asset name (e.g. {@code SHA256SUMS.txt})
   * @param githubRepo repository (owner/repo)
   * @return the download URL
   */
  public String releaseDownloadUrl(String version, String asset, String githubRepo) {
    return downloadBaseUrl + "/" + githubRepo + "/releases/download/" + version + "/" + asset;
  }

  /**
   * The tag of a repository's latest published release.
   *
   * @param githubRepo repository (owner/repo)
   * @return the release tag (e.g. {@code v0.3.0})
   * @throws ServiceStartException if the release cannot be queried or carries no tag
   */
  public String resolveLatestVersion(String githubRepo) {
    String url = apiBaseUrl + "/repos/" + githubRepo + "/releases/latest";
    String tag;
    try {
      Object release = Json.parse(new String(get(url), StandardCharsets.UTF_8));
      tag = Json.stringMember(release, "tag_name");
    } catch (IOException | InterruptedException | IllegalArgumentException e) {
      if (e instanceof InterruptedException) {
        Thread.currentThread().interrupt();
      }
      throw new ServiceStartException("failed to resolve latest release from " + url + ": " + e);
    }
    if (tag.isEmpty()) {
      throw new ServiceStartException("latest release of " + githubRepo + " has no tag name");
    }
    return tag;
  }

  /**
   * The digest a download must have, from the pin or the signed manifest.
   *
   * <p>The pin wins wherever there is one, and a verified manifest contradicting it is a failure
   * rather than a reason to prefer either. Where there is no pin, a digest taken from a manifest
   * signed by the release pipeline stands in for one; nothing else does.
   *
   * @param version release tag, resolved (never {@code latest})
   * @param asset asset name
   * @param servedDigest digest from the {@code .sha256} served beside the binary
   * @param githubRepo repository (owner/repo)
   * @param verifiedDigest digest from the release's signed checksum manifest, or {@code null}
   * @param unverifiedReason why there is no verified digest, said in the refusal when there is no
   *     pin either, or {@code null}
   * @param options how the connection was configured
   * @return the digest to verify the download against
   * @throws ChecksumMismatchException if the served digest or a verified manifest contradicts the
   *     pinned one
   * @throws UnpinnedReleaseException if nothing is pinned for it, no signed manifest vouched for
   *     it, and same-origin trust was not allowed explicitly
   */
  public String expectedDigest(
      String version,
      String asset,
      String servedDigest,
      String githubRepo,
      String verifiedDigest,
      String unverifiedReason,
      ConnectionOptions options) {
    Optional<String> pinned = pinnedDigest(version, asset, githubRepo);
    if (pinned.isEmpty()) {
      if (verifiedDigest != null) {
        // Signed by the pipeline that built the release, so it is not the origin vouching for
        // itself: as good as a pin.
        return verifiedDigest;
      }
      if (!unpinnedDownloadsAllowed(options, githubRepo, environment)) {
        throw new UnpinnedReleaseException(
            "this client pins no SHA-256 digest for "
                + asset
                + " of "
                + version
                + " of "
                + githubRepo
                + (unverifiedReason == null ? "" : " and " + unverifiedReason)
                + ", so the only checksum available is the one served beside the binary, which a "
                + "compromised release would serve too. Upgrade to a client release that pins "
                + version
                + ", ask for a pinned release with ConnectionOptions.downloadVersion(), or accept "
                + "same-origin trust for this repository by setting $"
                + ConnectionOptions.ALLOW_UNPINNED_ENV
                + "="
                + githubRepo
                + " (or =1 for any repository).");
      }
      warn(
          "this client pins no digest for "
              + asset
              + " of "
              + version
              + " of "
              + githubRepo
              + "; verifying it against the checksum served beside it, which detects corruption "
              + "but not a compromised release (same-origin trust was allowed explicitly).");
      return servedDigest;
    }
    if (verifiedDigest != null && !verifiedDigest.equals(pinned.get())) {
      throw new ChecksumMismatchException(
          "checksum mismatch for "
              + asset
              + " of "
              + version
              + ": the signed "
              + SignedManifest.MANIFEST_ASSET
              + " of "
              + githubRepo
              + " lists "
              + verifiedDigest
              + ", but this client pins "
              + pinned.get()
              + ". The release was rebuilt after this client pinned it; it was not installed.");
    }
    if (!servedDigest.equals(pinned.get())) {
      throw new ChecksumMismatchException(
          "checksum mismatch for "
              + asset
              + " of "
              + version
              + ": "
              + githubRepo
              + " serves "
              + servedDigest
              + ", but this client pins "
              + pinned.get()
              + ". The release was republished with another binary, or the download is being "
              + "tampered with; it was not installed.");
    }
    return pinned.get();
  }

  /**
   * The digest for an asset from the release's signed checksum manifest.
   *
   * @param version release tag, resolved (never {@code latest})
   * @param asset asset name
   * @param githubRepo repository (owner/repo)
   * @return the hex digest, from a manifest the release pipeline signed
   * @throws UnsignedReleaseException if the release publishes no signature this client can check
   * @throws io.opensysml.ManifestSignatureException if the signature does not verify
   */
  public String signedManifestDigest(String version, String asset, String githubRepo) {
    ReleaseSigner signer =
        SignedManifest.signerFor(githubRepo)
            .orElseThrow(
                () ->
                    new UnsignedReleaseException(
                        "this client knows no release pipeline identity for "
                            + githubRepo
                            + ", so a signed "
                            + SignedManifest.MANIFEST_ASSET
                            + " of it would not be verifiable"));

    byte[] manifest = signedAsset(version, SignedManifest.MANIFEST_ASSET, githubRepo);
    byte[] bundle = signedAsset(version, SignedManifest.BUNDLE_ASSET, githubRepo);
    return SignedManifest.verifiedDigest(manifest, bundle, asset, signer, trustedRoot);
  }

  /**
   * Downloads the release binary for this platform and installs it in the cache.
   *
   * @param version release tag, or {@code latest}
   * @param options how the connection was configured
   * @return the installed binary
   * @throws ChecksumMismatchException if the download does not have the digest expected of it
   * @throws UnpinnedReleaseException if nothing vouches for the release
   * @throws ServiceStartException if the download or its installation fails
   */
  public Path downloadBinary(String version, ConnectionOptions options) {
    String githubRepo = githubRepo(options);
    if ("latest".equals(version)) {
      version = resolveLatestVersion(githubRepo);
    }
    String asset = this.asset == null ? ReleasePlatform.assetName() : this.asset;
    String binaryUrl = releaseDownloadUrl(version, asset, githubRepo);

    // A release nothing is pinned for is vouched for by the signature on its checksum
    // manifest instead, which the origin cannot forge.
    String verifiedChecksum = null;
    String unverifiedReason = null;
    if (pinnedDigest(version, asset, githubRepo).isEmpty()) {
      try {
        verifiedChecksum = signedManifestDigest(version, asset, githubRepo);
      } catch (UnsignedReleaseException e) {
        unverifiedReason = e.getMessage();
      }
    }

    String servedChecksum = servedDigest(version, asset, githubRepo);
    String expected =
        expectedDigest(
            version,
            asset,
            servedChecksum,
            githubRepo,
            verifiedChecksum,
            unverifiedReason,
            options);

    byte[] downloaded = fetch(binaryUrl, "binary");
    install(downloaded, expected, asset, version, githubRepo);
    return binaryPath;
  }

  /**
   * Why the cached binary is not the release that was asked for.
   *
   * <p>A cache left by an earlier release otherwise serves a client asking for a newer one, which
   * then fails on whatever that build cannot do.
   *
   * @param version release tag asked for, or {@code latest}
   * @param githubRepo repository (owner/repo)
   * @return why the cache does not answer for that release, or empty when it does
   */
  public Optional<String> staleCacheReason(String version, String githubRepo) {
    String wanted = version;
    if ("latest".equals(wanted)) {
      try {
        wanted = resolveLatestVersion(githubRepo);
      } catch (ServiceStartException e) {
        // Unreachable releases are no reason to discard a working cache.
        return Optional.empty();
      }
    }

    Optional<String> have = cachedRelease(githubRepo);
    if (have.isPresent()) {
      return have.get().equals(wanted)
          ? Optional.empty()
          : Optional.of(
              "the binary cached at " + binaryPath + " is " + have.get() + ", but " + wanted
                  + " was asked for");
    }
    String recordedRepo = Json.stringMember(readMetadata(), "repo");
    if (!recordedRepo.isEmpty() && !recordedRepo.equals(githubRepo)) {
      return Optional.of(
          "the binary cached at " + binaryPath + " was downloaded from " + recordedRepo + ", but "
              + wanted + " of " + githubRepo + " was asked for");
    }
    return Optional.of(
        "the binary cached at " + binaryPath + " was not downloaded by this client, so which "
            + "release it is cannot be told, and " + wanted + " was asked for");
  }

  /**
   * The release the cached binary was downloaded from, if it was downloaded from this repository.
   *
   * <p>The digest is re-checked, so a binary swapped in by hand is not read as the release it
   * displaced. Forks publish the same tags, so a cache from another repository does not answer for
   * this one either.
   *
   * @param githubRepo repository (owner/repo) asked about
   * @return the release tag, or empty when the cache cannot be vouched for
   */
  public Optional<String> cachedRelease(String githubRepo) {
    Object recorded = readMetadata();
    String version = Json.stringMember(recorded, "version");
    String digest = Json.stringMember(recorded, "sha256");
    if (version.isEmpty() || digest.isEmpty()) {
      return Optional.empty();
    }
    // A record without a repository predates one being kept, so it says nothing.
    if (!Json.stringMember(recorded, "repo").equals(githubRepo)) {
      return Optional.empty();
    }
    try {
      if (!sha256(binaryPath).equalsIgnoreCase(digest)) {
        return Optional.empty();
      }
    } catch (IOException e) {
      return Optional.empty();
    }
    return Optional.of(version);
  }

  /** Records which release of which repository the cached binary is, and its digest. */
  private void writeMetadata(String version, String sha256, String githubRepo) {
    String json =
        "{\"version\": "
            + quote(version)
            + ", \"sha256\": "
            + quote(sha256)
            + ", \"repo\": "
            + quote(githubRepo)
            + "}";
    try {
      Files.writeString(metadataPath(), json, StandardCharsets.UTF_8);
    } catch (IOException e) {
      // The binary is installed; a cache whose release cannot be recorded is only re-downloaded.
      warn("could not record the release of " + binaryPath + ": " + e);
    }
  }

  private Object readMetadata() {
    try {
      return Json.parse(Files.readString(metadataPath(), StandardCharsets.UTF_8));
    } catch (IOException | IllegalArgumentException e) {
      return null;
    }
  }

  /** Writes the download to a temporary file, verifies it, and only then replaces the cache. */
  private void install(
      byte[] downloaded, String expected, String asset, String version, String githubRepo) {
    Path temporary = binaryPath.resolveSibling(binaryPath.getFileName() + ".tmp");
    try {
      Files.createDirectories(binaryPath.getParent());
      Files.write(temporary, downloaded);
    } catch (IOException e) {
      deleteQuietly(temporary);
      throw new ServiceStartException(
          "downloaded " + version + " but could not write it at " + temporary + ": " + e);
    }

    String actual;
    try {
      actual = sha256(temporary);
    } catch (IOException e) {
      deleteQuietly(temporary);
      throw new ServiceStartException(
          "downloaded " + version + " but could not hash it at " + temporary + ": " + e);
    }
    if (!actual.equalsIgnoreCase(expected)) {
      deleteQuietly(temporary);
      throw new ChecksumMismatchException(
          "checksum mismatch for "
              + asset
              + ". Expected "
              + expected
              + ", but the download hashes to "
              + actual
              + ". It may be corrupted or tampered with; it was not installed.");
    }

    // The cache is this user's, so no one else needs the binary; set it before the move so
    // nothing ever sees a world-readable one at the cached path.
    makeExecutable(temporary);
    try {
      move(temporary, binaryPath);
    } catch (IOException e) {
      deleteQuietly(temporary);
      throw new ServiceStartException(
          "downloaded "
              + version
              + " but could not install it at "
              + binaryPath
              + ": "
              + e
              + ". A running service holding that file is the usual cause.");
    }
    writeMetadata(version, expected.toLowerCase(Locale.ROOT), githubRepo);
  }

  private static void move(Path from, Path to) throws IOException {
    try {
      Files.move(from, to, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
    } catch (AtomicMoveNotSupportedException e) {
      Files.move(from, to, StandardCopyOption.REPLACE_EXISTING);
    }
  }

  /** Mode 0700 where the filesystem keeps POSIX permissions, the executable bit where it does not. */
  private static void makeExecutable(Path path) {
    try {
      Files.setPosixFilePermissions(
          path,
          Set.of(
              PosixFilePermission.OWNER_READ,
              PosixFilePermission.OWNER_WRITE,
              PosixFilePermission.OWNER_EXECUTE));
      return;
    } catch (UnsupportedOperationException | IOException e) {
      // Windows and some network filesystems keep no POSIX mode.
    }
    if (!path.toFile().setExecutable(true, true) && !path.toFile().canExecute()) {
      throw new ServiceStartException("downloaded sysml-grpc at " + path + " could not be made "
          + "executable");
    }
  }

  /** The digest the release serves beside the binary, which only the origin vouches for. */
  private String servedDigest(String version, String asset, String githubRepo) {
    String url = releaseDownloadUrl(version, asset + ".sha256", githubRepo);
    String served = new String(fetch(url, "checksum"), StandardCharsets.UTF_8).trim();
    String[] fields = served.split("\\s+");
    if (fields.length == 0 || !fields[0].toLowerCase(Locale.ROOT).matches("[0-9a-f]{64}")) {
      throw new ServiceStartException(
          "the checksum served for " + asset + " of " + version + " at " + url
              + " is not a SHA-256 digest");
    }
    return fields[0].toLowerCase(Locale.ROOT);
  }

  /** An asset of the signed manifest; an unreadable one leaves the release simply unverified. */
  private byte[] signedAsset(String version, String asset, String githubRepo) {
    String url = releaseDownloadUrl(version, asset, githubRepo);
    try {
      return get(url);
    } catch (IOException | InterruptedException | ServiceStartException e) {
      if (e instanceof InterruptedException) {
        Thread.currentThread().interrupt();
      }
      throw new UnsignedReleaseException(
          version + " of " + githubRepo + " publishes no readable " + asset + " (" + url + ": " + e
              + "), so its checksums carry no signature to verify");
    }
  }

  private byte[] fetch(String url, String what) {
    try {
      return get(url);
    } catch (IOException | InterruptedException e) {
      if (e instanceof InterruptedException) {
        Thread.currentThread().interrupt();
      }
      throw new ServiceStartException("failed to download the " + what + " from " + url + ": " + e);
    }
  }

  private byte[] get(String url) throws IOException, InterruptedException {
    HttpRequest request =
        HttpRequest.newBuilder(URI.create(url))
            .timeout(NETWORK_TIMEOUT)
            .header("Accept", "*/*")
            .GET()
            .build();
    HttpResponse<byte[]> response = http.send(request, HttpResponse.BodyHandlers.ofByteArray());
    if (response.statusCode() != 200) {
      throw new ServiceStartException(url + " answered HTTP " + response.statusCode());
    }
    return response.body();
  }

  private void warn(String message) {
    warnings.accept(message);
  }

  private static void deleteQuietly(Path path) {
    try {
      Files.deleteIfExists(path);
    } catch (IOException e) {
      // A temporary file that cannot be removed is not worth failing an install over.
    }
  }

  private static String quote(String value) {
    StringBuilder quoted = new StringBuilder(value.length() + 2).append('"');
    for (int i = 0; i < value.length(); i++) {
      char c = value.charAt(i);
      if (c == '"' || c == '\\') {
        quoted.append('\\').append(c);
      } else if (c < 0x20) {
        quoted.append(String.format(Locale.ROOT, "\\u%04x", (int) c));
      } else {
        quoted.append(c);
      }
    }
    return quoted.append('"').toString();
  }

  private static String sha256(Path file) throws IOException {
    MessageDigest digest;
    try {
      digest = MessageDigest.getInstance("SHA-256");
    } catch (NoSuchAlgorithmException e) {
      throw new IllegalStateException("SHA-256 is required of every JVM", e);
    }
    try (InputStream in = Files.newInputStream(file)) {
      byte[] buffer = new byte[65536];
      int read;
      while ((read = in.read(buffer)) > 0) {
        digest.update(buffer, 0, read);
      }
    }
    return HexFormat.of().formatHex(digest.digest());
  }

  /** Builds a {@link BinaryDownloader}; the defaults are the real GitHub and the shipped pins. */
  static final class Builder {

    private String downloadBaseUrl = "https://github.com";
    private String apiBaseUrl = "https://api.github.com";
    private Path binaryPath = ReleasePlatform.cachedBinary();
    private ReleaseDigests pins = ReleaseDigests.shipped();
    private Path trustedRoot;
    private String asset;
    private UnaryOperator<String> environment = System::getenv;
    private Consumer<String> warnings = message -> LOG.log(System.Logger.Level.WARNING, message);

    Builder downloadBaseUrl(String downloadBaseUrl) {
      this.downloadBaseUrl = downloadBaseUrl;
      return this;
    }

    Builder apiBaseUrl(String apiBaseUrl) {
      this.apiBaseUrl = apiBaseUrl;
      return this;
    }

    Builder binaryPath(Path binaryPath) {
      this.binaryPath = binaryPath;
      return this;
    }

    Builder pins(ReleaseDigests pins) {
      this.pins = pins;
      return this;
    }

    /** A sigstore trusted root to verify against, for an offline fixture of a signed release. */
    Builder trustedRoot(Path trustedRoot) {
      this.trustedRoot = trustedRoot;
      return this;
    }

    Builder environment(UnaryOperator<String> environment) {
      this.environment = environment;
      return this;
    }

    /** The asset to download, for a test whose fixtures cover one platform. */
    Builder asset(String asset) {
      this.asset = asset;
      return this;
    }

    Builder warnings(Consumer<String> warnings) {
      this.warnings = warnings;
      return this;
    }

    BinaryDownloader build() {
      return new BinaryDownloader(this);
    }
  }
}
