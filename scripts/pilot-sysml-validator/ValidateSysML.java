import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

import org.eclipse.emf.common.util.URI;
import org.eclipse.emf.ecore.EPackage;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.xtext.diagnostics.Severity;
import org.eclipse.xtext.util.CancelIndicator;
import org.eclipse.xtext.validation.CheckMode;
import org.eclipse.xtext.validation.IResourceValidator;
import org.eclipse.xtext.validation.Issue;
import org.omg.kerml.xtext.KerMLStandaloneSetup;
import org.omg.kerml.xtext.xmi.KerMLxStandaloneSetup;
import org.omg.sysml.io.SysMLUtil;
import org.omg.sysml.lang.sysml.SysMLPackage;
import org.omg.sysml.util.SysMLLibraryUtil;
import org.omg.sysml.xtext.SysMLStandaloneSetup;
import org.omg.sysml.xtext.xmi.SysMLxStandaloneSetup;

import com.google.inject.Injector;

/**
 * Runs the pilot's own SysMLValidator over .sysml files loaded into one resource set,
 * reporting Xtext issues on stderr in GNU format: file:line:column: severity: message.
 *
 * <p>The SysML twin of ValidateKerML: a genuine batch load, as opposed to the accumulating
 * SysMLInteractive session the DeciSym CLI drives one file at a time.
 */
public final class ValidateSysML extends SysMLUtil {

    private static final String KERML_EXTENSION = ".kerml";
    private static final String SYSML_EXTENSION = ".sysml";

    private static final String LIBRARY_DIRECTORY_NAME = "sysml.library";
    private static final String KERNEL_LIBRARIES_DIRECTORY = "Kernel Libraries";
    private static final String SYSTEMS_LIBRARY_DIRECTORY = "Systems Library";
    private static final String DOMAIN_LIBRARIES_DIRECTORY = "Domain Libraries";

    private static final Pattern OBJECT_REFERENCE =
            Pattern.compile("([\\w.$]+)@[0-9a-f]+\\{file:([^#{}]*)#([^{}]*)\\}");

    private final IResourceValidator validator;
    private final Path root;
    private final List<Input> inputs = new ArrayList<>();
    private boolean hasErrors = false;

    private static final class Input {
        private final Path file;
        private final Resource resource;

        private Input(Path file, Resource resource) {
            this.file = file;
            this.resource = resource;
        }
    }

    private ValidateSysML(IResourceValidator validator, Path root) {
        super();
        this.validator = validator;
        this.root = root;
        setVerbose(false);
    }

    /** Load the standard library the way the pilot's own interactive session does. */
    private void loadLibrary(Path library) {
        SysMLLibraryUtil.setModelLibraryDirectory(library.toString());
        readAll(library.resolve(KERNEL_LIBRARIES_DIRECTORY).toString(), false, KERML_EXTENSION);
        readAll(library.resolve(SYSTEMS_LIBRARY_DIRECTORY).toString(), false, SYSML_EXTENSION);
        readAll(library.resolve(DOMAIN_LIBRARIES_DIRECTORY).toString(), false, SYSML_EXTENSION);
    }

    /** Read every input file into the shared resource set and the Xtext index. */
    private void readInputs(List<Path> files) {
        for (Path file : files) {
            try {
                Resource resource = readResource(file.toString());
                addInputResource(resource);
                inputs.add(new Input(file, resource));
            } catch (RuntimeException e) {
                report(file, 1, 1, Severity.ERROR, message(e));
            }
        }
    }

    private void validateInputs() {
        for (Input input : inputs) {
            try {
                for (Issue issue : validator.validate(input.resource, CheckMode.ALL, CancelIndicator.NullImpl)) {
                    int line = issue.getLineNumber() == null ? 1 : issue.getLineNumber();
                    int column = issue.getColumn() == null ? 1 : issue.getColumn();
                    report(input.file, line, column, issue.getSeverity(), issue.getMessage());
                }
            } catch (RuntimeException e) {
                report(input.file, 1, 1, Severity.ERROR, message(e));
            }
        }
    }

    /** Print one diagnostic in GNU format, collapsing embedded newlines. */
    private void report(Path file, int line, int column, Severity severity, String message) {
        String name = display(file);
        String text = message == null ? "" : normalize(message.replaceAll("\\s+", " ").trim());
        System.err.println(name + ":" + line + ":" + column + ": "
                + severity.toString().toLowerCase() + ": " + text);
        if (severity == Severity.ERROR) {
            hasErrors = true;
        }
    }

    /** Drop identity hash codes and absolute URIs from EMF object references. */
    private String normalize(String message) {
        Matcher matcher = OBJECT_REFERENCE.matcher(message);
        StringBuilder result = new StringBuilder();
        while (matcher.find()) {
            String path = URI.createURI("file:" + matcher.group(2)).toFileString();
            String name = path == null ? matcher.group(2) : display(Paths.get(path));
            String replacement = matcher.group(1) + "{" + name + "#" + matcher.group(3) + "}";
            matcher.appendReplacement(result, Matcher.quoteReplacement(replacement));
        }
        matcher.appendTail(result);
        return result.toString();
    }

    /** Root-relative path where possible, else a library- or name-relative one. */
    private String display(Path file) {
        if (root != null && file.startsWith(root)) {
            return root.relativize(file).toString();
        }
        int library = file.getNameCount();
        for (int i = 0; i < file.getNameCount(); i++) {
            if (file.getName(i).toString().equals(LIBRARY_DIRECTORY_NAME)) {
                library = i;
                break;
            }
        }
        if (library < file.getNameCount()) {
            return file.subpath(library, file.getNameCount()).toString();
        }
        return file.getFileName().toString();
    }

    private static String message(Throwable e) {
        String text = e.getMessage();
        return text == null ? e.getClass().getName() : text;
    }

    private static List<Path> collect(List<String> arguments) throws IOException {
        List<Path> files = new ArrayList<>();
        for (String argument : arguments) {
            Path path = Paths.get(argument).toAbsolutePath().normalize();
            if (Files.isDirectory(path)) {
                try (Stream<Path> walk = Files.walk(path)) {
                    walk.filter(Files::isRegularFile)
                            .filter(p -> p.toString().endsWith(SYSML_EXTENSION))
                            .sorted()
                            .forEach(files::add);
                }
            } else {
                files.add(path);
            }
        }
        return files;
    }

    private static void usage() {
        System.err.println("usage: validate-sysml --library DIR [--root DIR] FILE...");
    }

    public static void main(String[] args) {
        try {
            Path library = null;
            Path root = null;
            List<String> inputs = new ArrayList<>();
            for (int i = 0; i < args.length; i++) {
                switch (args[i]) {
                    case "--library" -> {
                        if (i + 1 >= args.length || args[i + 1].startsWith("--")) {
                            throw new IllegalArgumentException("--library requires a value");
                        }
                        library = Paths.get(args[++i]).toAbsolutePath().normalize();
                    }
                    case "--root" -> {
                        if (i + 1 >= args.length || args[i + 1].startsWith("--")) {
                            throw new IllegalArgumentException("--root requires a value");
                        }
                        root = Paths.get(args[++i]).toAbsolutePath().normalize();
                    }
                    case "-h", "--help" -> {
                        usage();
                        System.exit(0);
                    }
                    default -> inputs.add(args[i]);
                }
            }

            if (library == null) {
                String property = System.getProperty("sysml.library");
                library = property == null ? null : Paths.get(property).toAbsolutePath().normalize();
            }
            if (library == null || !Files.isDirectory(library)) {
                System.err.println("Error: SysML library not found: " + library);
                usage();
                System.exit(2);
            }
            if (inputs.isEmpty()) {
                usage();
                System.exit(2);
            }

            List<Path> files = collect(inputs);
            for (Path file : files) {
                if (!file.toString().endsWith(SYSML_EXTENSION)) {
                    System.err.println("Error: File must have .sysml extension: " + file);
                    System.exit(2);
                }
                if (!Files.isRegularFile(file)) {
                    System.err.println("Error: File not found: " + file);
                    System.exit(2);
                }
            }
            if (files.isEmpty()) {
                System.err.println("Warning: No .sysml files found");
                System.exit(0);
            }

            // The same registration sequence SysMLInteractive.createInstance performs, so the
            // only difference from the interactive path is how resources are loaded.
            EPackage.Registry.INSTANCE.put(
                    "https://www.omg.org/spec/SysML/20250201", SysMLPackage.eINSTANCE);
            KerMLStandaloneSetup.doSetup();
            KerMLxStandaloneSetup.doSetup();
            SysMLxStandaloneSetup.doSetup();
            Injector injector = new SysMLStandaloneSetup().createInjectorAndDoEMFRegistration();
            IResourceValidator validator = injector.getInstance(IResourceValidator.class);

            ValidateSysML instance = new ValidateSysML(validator, root);
            instance.loadLibrary(library);
            instance.readInputs(files);
            instance.validateInputs();
            System.exit(instance.hasErrors ? 1 : 0);
        } catch (Throwable e) {
            System.err.println("Error: " + message(e));
            System.exit(3);
        }
    }
}
