package io.opensysml.internal;

import io.opensysml.ConnectionOptions;
import java.io.IOException;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.locks.ReentrantLock;

/**
 * The private services this copy of the client owns, shared by the binary they run.
 *
 * <p>One child per classloader that loaded this class, which is the JVM analogue of the Python
 * client's one child per interpreter: the state is static, so an Eclipse plugin, a web application
 * and a shaded copy inside a third library each get their own, while every connection made by one
 * of them shares a service and therefore its parse cache. A host that must not let one tenant's
 * models sit in another tenant's cache asks for
 * {@link ConnectionOptions.Builder#isolatedService(boolean)} instead, which bypasses this registry.
 *
 * <p>Every field is guarded by one lock: connections are made concurrently by host applications,
 * and two threads racing here would either start two children or leave a child with a reference
 * count that never reaches zero.
 */
public final class ServiceRegistry {

  private static final ReentrantLock LOCK = new ReentrantLock();
  private static final Map<String, PrivateService> SHARED = new HashMap<>();

  private ServiceRegistry() {}

  /**
   * The shared service for a configuration, started if it is not running yet, with a reference
   * taken on it.
   *
   * @param options how the connection was configured
   * @return a running service the caller must {@link #release(PrivateService)}
   */
  public static PrivateService acquire(ConnectionOptions options) {
    Path binary = BinaryResolver.resolve(options);
    Duration startupTimeout = options.startupTimeout();
    if (options.isolatedService()) {
      PrivateService isolated = PrivateService.start(binary, startupTimeout);
      LOCK.lock();
      try {
        isolated.retain();
      } finally {
        LOCK.unlock();
      }
      return isolated;
    }

    String key = key(binary);
    LOCK.lock();
    try {
      PrivateService service = SHARED.get(key);
      if (service != null && service.isAlive()) {
        service.retain();
        return service;
      }
      if (service != null) {
        // It died on its own; forget it rather than hand out a dead address.
        SHARED.remove(key);
      }
    } finally {
      LOCK.unlock();
    }

    // Started outside the lock so a slow start does not block every other connection.
    PrivateService started = PrivateService.start(binary, startupTimeout);
    LOCK.lock();
    try {
      PrivateService raced = SHARED.get(key);
      if (raced != null && raced.isAlive()) {
        started.stop();
        raced.retain();
        return raced;
      }
      started.retain();
      SHARED.put(key, started);
      return started;
    } finally {
      LOCK.unlock();
    }
  }

  /**
   * Gives up one reference, stopping the service when it was the last.
   *
   * @param service a service acquired here
   */
  public static void release(PrivateService service) {
    boolean stop = false;
    LOCK.lock();
    try {
      if (service.release() <= 0) {
        SHARED.values().removeIf(shared -> shared == service);
        stop = true;
      }
    } finally {
      LOCK.unlock();
    }
    if (stop) {
      service.stop();
    }
  }

  /**
   * Stops every shared service, whatever their reference counts, for a host unloading the client.
   *
   * @return how many services were stopped
   */
  public static int stopAll() {
    List<PrivateService> stopping;
    LOCK.lock();
    try {
      stopping = new ArrayList<>(SHARED.values());
      SHARED.clear();
    } finally {
      LOCK.unlock();
    }
    for (PrivateService service : stopping) {
      service.stop();
    }
    return stopping.size();
  }

  /**
   * How many shared services are running, for a test.
   *
   * @return the number of shared services
   */
  public static int sharedServiceCount() {
    LOCK.lock();
    try {
      return SHARED.size();
    } finally {
      LOCK.unlock();
    }
  }

  private static String key(Path binary) {
    try {
      return binary.toRealPath().toString();
    } catch (IOException e) {
      // Unreadable is not a reason to start a second child for the same name.
      return binary.toAbsolutePath().toString();
    }
  }
}
