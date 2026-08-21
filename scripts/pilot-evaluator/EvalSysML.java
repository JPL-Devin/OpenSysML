import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;

import org.omg.sysml.interactive.SysMLInteractive;
import org.omg.sysml.interactive.SysMLInteractiveResult;

/** Drives the pilot's model-level expression evaluator over evaluation requests. */
public final class EvalSysML {

    private static final String CASE_MARKER = "== case ";
    private static final String END_MARKER = "== end ";
    private static final String MODEL_MARKER = "== model ";

    /** One evaluation request: id, optional target, and expression. */
    private record Request(String id, String target, String expr) {
    }

    private final SysMLInteractive instance;

    private EvalSysML(SysMLInteractive instance) {
        this.instance = instance;
    }

    /** Process each model into the accumulating session. */
    private void loadModels(List<Path> models) throws IOException {
        for (Path model : models) {
            SysMLInteractiveResult result = instance.process(Files.readString(model));
            System.out.println(MODEL_MARKER + model.getFileName());
            emit(result.toString());
            System.out.println(END_MARKER + model.getFileName());
        }
    }

    private void evaluate(List<Request> requests) {
        for (Request request : requests) {
            String output;
            try {
                output = instance.eval(request.expr(), request.target(), List.of());
            } catch (RuntimeException | StackOverflowError e) {
                output = "EXCEPTION:" + e.getClass().getName() + ": " + e.getMessage();
            }
            System.out.println(CASE_MARKER + request.id());
            emit(output);
            System.out.println(END_MARKER + request.id());
        }
    }

    /** Print output between markers, escaping marker-looking result lines. */
    private static void emit(String output) {
        if (output == null || output.isEmpty()) {
            return;
        }
        for (String line : output.split("\n", -1)) {
            if (line.isEmpty()) {
                continue;
            }
            System.out.println(line.startsWith("== ") ? " " + line : line);
        }
    }

    /** Parse tab-separated evaluation requests. */
    private static List<Request> readRequests(Path file) throws IOException {
        List<Request> requests = new ArrayList<>();
        int number = 0;
        for (String line : Files.readAllLines(file)) {
            number++;
            if (line.isBlank() || line.startsWith("#")) {
                continue;
            }
            String[] fields = line.split("\t", -1);
            if (fields.length != 3) {
                throw new IllegalArgumentException(
                        file + ":" + number + ": expected id<TAB>target<TAB>expression");
            }
            String id = fields[0].trim();
            String target = fields[1].trim();
            String expr = fields[2];
            if (id.isEmpty() || expr.isBlank()) {
                throw new IllegalArgumentException(file + ":" + number + ": empty id or expression");
            }
            requests.add(new Request(id, target.isEmpty() ? null : target, expr));
        }
        return requests;
    }

    private static void usage() {
        System.err.println("usage: eval-sysml --library DIR --cases FILE [--model FILE]...");
    }

    private static String message(Throwable e) {
        String text = e.getMessage();
        return text == null ? e.getClass().getName() : text;
    }

    public static void main(String[] args) {
        try {
            Path library = null;
            Path cases = null;
            List<Path> models = new ArrayList<>();
            for (int i = 0; i < args.length; i++) {
                switch (args[i]) {
                    case "--library" -> library = value(args, ++i, "--library");
                    case "--cases" -> cases = value(args, i + 1 < args.length ? ++i : i, "--cases");
                    case "--model" -> models.add(value(args, i + 1 < args.length ? ++i : i, "--model"));
                    case "-h", "--help" -> {
                        usage();
                        System.exit(0);
                    }
                    default -> throw new IllegalArgumentException("unknown argument: " + args[i]);
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
            if (cases == null || !Files.isRegularFile(cases)) {
                System.err.println("Error: case file not found: " + cases);
                usage();
                System.exit(2);
            }
            for (Path model : models) {
                if (!Files.isRegularFile(model)) {
                    System.err.println("Error: model not found: " + model);
                    System.exit(2);
                }
            }

            List<Request> requests = readRequests(cases);
            if (requests.isEmpty()) {
                System.err.println("Error: no evaluation requests in " + cases);
                System.exit(2);
            }

            SysMLInteractive instance = SysMLInteractive.createInstance();
            instance.loadLibrary(library.toString());
            EvalSysML driver = new EvalSysML(instance);
            driver.loadModels(models);
            driver.evaluate(requests);
            System.exit(0);
        } catch (Throwable e) {
            System.err.println("Error: " + message(e));
            System.exit(3);
        }
    }

    private static Path value(String[] args, int index, String option) {
        if (index >= args.length || args[index].startsWith("--")) {
            throw new IllegalArgumentException(option + " requires a value");
        }
        return Paths.get(args[index]).toAbsolutePath().normalize();
    }
}
