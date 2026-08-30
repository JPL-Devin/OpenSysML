package io.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.opensysml.ChecksumMismatchException;
import io.opensysml.ConnectionOptions;
import io.opensysml.ServiceStartException;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** When resolving a binary downloads one, and when it uses what is already installed. */
class BinaryResolverDownloadTest {

  private static final String REPO = "Open-MBEE/OpenSysML";
  private static final String VERSION = "v9.9.9";
  private static final String ASSET = "sysml-grpc-linux-amd64";
  private static final byte[] BINARY = "#!/bin/sh\nexit 0\n".getBytes(StandardCharsets.UTF_8);

  @TempDir Path home;

  private ReleaseServer releases;

  @BeforeEach
  void serveRelease() throws IOException {
    releases = new ReleaseServer();
  }

  @AfterEach
  void stopServing() {
    releases.close();
  }

  private Path cache() {
    return home.resolve("bin").resolve("sysml-grpc");
  }

  private BinaryDownloader downloader() {
    return new BinaryDownloader.Builder()
        .downloadBaseUrl(releases.downloadBaseUrl())
        .apiBaseUrl(releases.apiBaseUrl())
        .binaryPath(cache())
        .asset(ASSET)
        .environment(name -> null)
        .pins(ReleaseDigests.of(Map.of(REPO, Map.of(VERSION, Map.of(ASSET, digest())))))
        .build();
  }

  private static String digest() {
    return ReleaseServer.sha256(BINARY);
  }

  @Test
  void downloadsTheReleaseAskedForWhenNothingIsInstalled() {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();

    Path resolved = BinaryResolver.resolve(options, downloader());

    assertEquals(cache(), resolved);
    assertTrue(Files.isExecutable(resolved));
  }

  @Test
  void usesTheCacheWhenItIsTheReleaseAskedFor() throws IOException {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();
    BinaryDownloader downloader = downloader();
    BinaryResolver.resolve(options, downloader);
    int fetched = releases.requested().size();

    Path resolved = BinaryResolver.resolve(options, downloader);

    assertEquals(cache(), resolved);
    assertEquals(fetched, releases.requested().size(), "a cache of that release is not re-fetched");
    assertTrue(Files.exists(downloader.metadataPath()));
  }

  @Test
  void refusesToStartWithoutABinaryWhenNoReleaseIsAskedFor() {
    ConnectionOptions options = ConnectionOptions.builder().binaryPath(cache()).build();
    ServiceStartException refused =
        assertThrows(ServiceStartException.class, () -> BinaryResolver.resolve(options,
            downloader()));
    assertTrue(refused.getMessage().contains("no executable sysml-grpc"), refused.getMessage());
  }

  @Test
  void aCallersPinnedDigestIsCheckedOnWhatWasDownloaded() {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions options =
        ConnectionOptions.builder()
            .downloadVersion(VERSION)
            .expectedBinarySha256("ab".repeat(32))
            .build();

    assertThrows(
        ChecksumMismatchException.class, () -> BinaryResolver.resolve(options, downloader()));
  }

  @Test
  void keepsAWorkingBinaryWhenTheReleaseAskedForCannotBeDownloaded() throws IOException {
    Files.createDirectories(cache().getParent());
    Files.writeString(cache(), "installed by hand");
    cache().toFile().setExecutable(true, true);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();

    Path resolved = BinaryResolver.resolve(options, downloader());

    assertEquals(cache(), resolved);
    assertEquals("installed by hand", Files.readString(resolved));
  }

  @Test
  void losesNothingWhenADownloadIsRefusedAsTamperedWith() throws IOException {
    releases.publish(REPO, VERSION, ASSET, "another binary".getBytes(StandardCharsets.UTF_8));
    Files.createDirectories(cache().getParent());
    Files.writeString(cache(), "installed by hand");
    cache().toFile().setExecutable(true, true);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();

    // A download contradicting a pin is evidence, so it is not answered from the cache.
    assertThrows(
        ChecksumMismatchException.class, () -> BinaryResolver.resolve(options, downloader()));
    assertEquals("installed by hand", Files.readString(cache()));
    assertFalse(Files.exists(cache().resolveSibling("sysml-grpc.tmp")));
  }
}
