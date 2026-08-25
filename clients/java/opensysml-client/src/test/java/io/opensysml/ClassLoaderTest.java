package io.opensysml;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.File;
import java.lang.ref.WeakReference;
import java.lang.reflect.Method;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;

/** A host application that loads and unloads the client repeatedly leaves nothing behind. */
class ClassLoaderTest {

  @Test
  void repeatedLoadAndUnloadLeavesNoChildAndNoUncollectableCopy() throws Exception {
    Path binary = ServiceBinary.required();
    List<Long> pids = new ArrayList<>();
    List<WeakReference<ClassLoader>> loaders = new ArrayList<>();

    for (int round = 0; round < 3; round++) {
      // Platform parent, so this copy of the client is a different class from the test's own.
      URLClassLoader loader =
          new URLClassLoader(
              "opensysml-host-" + round, classpath(), ClassLoader.getPlatformClassLoader());
      Method run = loader.loadClass(ClassLoaderProbe.class.getName()).getMethod("run", String.class);
      long pid = (long) run.invoke(null, binary.toString());
      pids.add(pid);
      loaders.add(new WeakReference<>(loader));
      loader.close();
    }

    // Each copy of the client owned its own child, since the registry is per classloader.
    assertEquals3DistinctChildren(pids);
    for (long pid : pids) {
      assertTrue(waitForExit(pid), "a child outlived the classloader that started it: " + pid);
    }
    assertTrue(
        nonDaemonThreads().isEmpty(), "an unloaded copy left a non-daemon thread: " + nonDaemonThreads());
    assertTrue(collected(loaders), "an unloaded copy of the client could not be collected");
  }

  private static void assertEquals3DistinctChildren(List<Long> pids) {
    assertFalse(pids.isEmpty());
    assertTrue(pids.stream().distinct().count() == pids.size(), "the copies shared a child: " + pids);
  }

  private static URL[] classpath() throws Exception {
    String[] entries = System.getProperty("java.class.path").split(File.pathSeparator);
    List<URL> urls = new ArrayList<>();
    for (String entry : entries) {
      urls.add(Path.of(entry).toUri().toURL());
    }
    return urls.toArray(new URL[0]);
  }

  private static boolean collected(List<WeakReference<ClassLoader>> loaders) throws Exception {
    for (int attempt = 0; attempt < 20; attempt++) {
      System.gc();
      Thread.sleep(100);
      if (loaders.stream().allMatch(reference -> reference.get() == null)) {
        return true;
      }
    }
    assertNull(loaders.get(0).get());
    return false;
  }

  private static boolean waitForExit(long pid) throws InterruptedException {
    for (int attempt = 0; attempt < 100; attempt++) {
      if (ProcessHandle.of(pid).filter(ProcessHandle::isAlive).isEmpty()) {
        return true;
      }
      Thread.sleep(100);
    }
    return false;
  }

  private static List<String> nonDaemonThreads() {
    return Thread.getAllStackTraces().keySet().stream()
        .filter(thread -> !thread.isDaemon())
        .filter(thread -> thread.getName().startsWith("opensysml"))
        .map(Thread::getName)
        .toList();
  }
}
