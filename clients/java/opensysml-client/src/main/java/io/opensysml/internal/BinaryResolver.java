package io.opensysml.internal;

import io.opensysml.ConnectionOptions;
import io.opensysml.ServiceStartException;
import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Optional;

/**
 * Finds the {@code sysml-grpc} binary a private service is started from.
 *
 * <p>This client never downloads one. The Python client can, because it pins a SHA-256 per release
 * asset and verifies a sigstore-signed checksum manifest; a Java client with no such pin would be
 * downloading over the network and executing what it got, so it does not. It runs a binary an
 * operator installed — {@code make build}, the Python client's verified download, or a release
 * asset checked against its signed manifest — and will verify a digest the caller pins with
 * {@link ConnectionOptions.Builder#expectedBinarySha256(String)} before executing it.
 */
public final class BinaryResolver {

  private BinaryResolver() {}

  /**
   * The binary to start, from the options, the environment, the user's cache, or {@code PATH}.
   *
   * @param options how the connection was configured
   * @return an existing, executable binary
   * @throws ServiceStartException if none was found, or a pinned digest does not match
   */
  public static Path resolve(ConnectionOptions options) {
    List<String> looked = new ArrayList<>();
    Optional<Path> found = options.binaryPath();
    if (found.isPresent()) {
      Path path = found.get();
      if (!isExecutable(path)) {
        throw new ServiceStartException("no executable sysml-grpc at " + path);
      }
      return verified(path, options);
    }
    for (Path candidate : candidates()) {
      looked.add(candidate.toString());
      if (isExecutable(candidate)) {
        return verified(candidate, options);
      }
    }
    throw new ServiceStartException(
        "no sysml-grpc binary found; build one with `make build`, install one with the "
            + "opensysml Python client, or name it with ConnectionOptions.binaryPath() or "
            + "$"
            + ConnectionOptions.BINARY_ENV
            + ". Looked at: "
            + String.join(", ", looked));
  }

  private static List<Path> candidates() {
    List<Path> candidates = new ArrayList<>();
    String named = System.getenv(ConnectionOptions.BINARY_ENV);
    if (named != null && !named.isBlank()) {
      candidates.add(Path.of(named.trim()));
    }
    String executable = isWindows() ? "sysml-grpc.exe" : "sysml-grpc";
    // Where the Python client installs its verified download, so the two share one binary.
    candidates.add(Path.of(System.getProperty("user.home"), ".opensysml", "bin", executable));
    String path = System.getenv("PATH");
    if (path != null) {
      for (String entry : path.split(File.pathSeparator)) {
        if (entry.isBlank()) {
          continue;
        }
        try {
          candidates.add(Path.of(entry, executable));
        } catch (InvalidPathException e) {
          // A PATH entry that is not a path is not a place to look.
        }
      }
    }
    return candidates;
  }

  private static Path verified(Path binary, ConnectionOptions options) {
    Optional<String> expected = options.expectedBinarySha256();
    if (expected.isEmpty()) {
      return binary;
    }
    String actual = sha256(binary);
    if (!actual.equalsIgnoreCase(expected.get())) {
      throw new ServiceStartException(
          "sysml-grpc at "
              + binary
              + " has SHA-256 "
              + actual
              + ", but "
              + expected.get()
              + " was required; it was not started");
    }
    return binary;
  }

  private static String sha256(Path binary) {
    try {
      MessageDigest digest = MessageDigest.getInstance("SHA-256");
      try (InputStream in = Files.newInputStream(binary)) {
        byte[] buffer = new byte[65536];
        int read;
        while ((read = in.read(buffer)) > 0) {
          digest.update(buffer, 0, read);
        }
      }
      StringBuilder hex = new StringBuilder(64);
      for (byte b : digest.digest()) {
        hex.append(String.format(Locale.ROOT, "%02x", b));
      }
      return hex.toString();
    } catch (IOException | NoSuchAlgorithmException e) {
      throw new ServiceStartException("sysml-grpc at " + binary + " could not be hashed", e);
    }
  }

  private static boolean isExecutable(Path path) {
    return Files.isRegularFile(path) && Files.isExecutable(path);
  }

  private static boolean isWindows() {
    return System.getProperty("os.name", "").toLowerCase(Locale.ROOT).startsWith("windows");
  }
}
