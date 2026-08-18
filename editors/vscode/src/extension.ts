import { accessSync, constants } from "node:fs";
import { delimiter, join } from "node:path";
import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

const EXECUTABLE = process.platform === "win32" ? "sysml-lsp.exe" : "sysml-lsp";

let client: LanguageClient | undefined;
let output: vscode.OutputChannel;
let watcher: vscode.FileSystemWatcher;
// Start/stop run one at a time: overlapping restarts would otherwise leave an
// unreferenced client, and its server process, running forever.
let queue: Promise<void> = Promise.resolve();

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  output = vscode.window.createOutputChannel("SysML v2");
  context.subscriptions.push(output);

  // One watcher for the whole session: a per-start watcher would outlive the
  // client that used it and pile up over restarts.
  watcher = vscode.workspace.createFileSystemWatcher("**/*.{sysml,kerml}");
  context.subscriptions.push(watcher);

  context.subscriptions.push(
    vscode.commands.registerCommand("opensysml.restartServer", () => restart()),
  );

  // The server binary is resolved at start, so pointing the setting at a fresh
  // build takes effect on the next restart rather than on reload.
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration("opensysml.server")) {
        void restart();
      }
    }),
  );

  await enqueue(startClient);
}

export async function deactivate(): Promise<void> {
  await enqueue(stopClient);
}

function restart(): Promise<void> {
  return enqueue(async () => {
    await stopClient();
    await startClient();
  });
}

// enqueue chains work onto the queue, so each step observes the previous one's
// result rather than a half-applied state.
function enqueue(work: () => Promise<void>): Promise<void> {
  queue = queue.then(work, work);
  return queue;
}

async function startClient(): Promise<void> {
  const config = vscode.workspace.getConfiguration("opensysml");
  if (!config.get<boolean>("server.enabled", true)) {
    output.appendLine("Language server disabled by opensysml.server.enabled; highlighting only.");
    return;
  }

  const command = resolveServer(config.get<string>("server.path", "").trim());
  if (!command) {
    void vscode.window.showWarningMessage(
      `Could not find ${EXECUTABLE}. Build it with \`make build\` and set "opensysml.server.path", or put it on your PATH. Syntax highlighting still works.`,
    );
    return;
  }
  output.appendLine(`Starting ${command}`);

  const args = config.get<string[]>("server.args", []);
  const serverOptions: ServerOptions = {
    run: { command, args, transport: TransportKind.stdio },
    debug: { command, args, transport: TransportKind.stdio },
  };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "sysml" },
      { scheme: "file", language: "kerml" },
    ],
    outputChannel: output,
    synchronize: { fileEvents: watcher },
  };

  client = new LanguageClient("opensysml", "SysML v2 Language Server", serverOptions, clientOptions);
  try {
    await client.start();
  } catch (err) {
    client = undefined;
    void vscode.window.showErrorMessage(`SysML v2 language server failed to start: ${String(err)}`);
  }
}

async function stopClient(): Promise<void> {
  const running = client;
  client = undefined;
  if (running) {
    await running.stop();
  }
}

// resolveServer prefers the configured path, then a build in the open
// workspace's bin/, then the executable on PATH.
function resolveServer(configured: string): string | undefined {
  if (configured) {
    return isExecutable(configured) ? configured : undefined;
  }
  for (const folder of vscode.workspace.workspaceFolders ?? []) {
    const candidate = join(folder.uri.fsPath, "bin", EXECUTABLE);
    if (isExecutable(candidate)) {
      return candidate;
    }
  }
  return onPath(EXECUTABLE);
}

function onPath(executable: string): string | undefined {
  for (const dir of (process.env.PATH ?? "").split(delimiter)) {
    if (dir && isExecutable(join(dir, executable))) {
      return join(dir, executable);
    }
  }
  return undefined;
}

function isExecutable(path: string): boolean {
  try {
    accessSync(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}
