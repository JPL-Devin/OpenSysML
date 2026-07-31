SysML-v2-Go — Handoff Notes
Project
New independent SysML v2 (incl KerML) language implementation in Go → goal: LSP server + CLI REPL. Module github.com/Open-MBEE/Systemica, root /home/han/IdeaProjects/Systems-Modeling, IS a git repo on master (env banner lies). Go 1.25. Pilot reference clone ./SysML-v2-Pilot-Implementation/ (gitignored, nested .git, NEVER commit; stdlib at sysml.library/, grammars at org.omg.*.xtext/). Spec: docs/superpowers/specs/2026-07-25-sysml-v2-go-design.md (15 sections, authoritative). docs/ dir untracked (intentional).
Status (HEAD 8ee50ee on master, all tests green + vet clean + -race clean)
- Plan 1 source+lexer — DONE
- Plan 2 ast+parser (namespace-core + full expressions; NO def/usage taxonomy) — DONE
- Plan 3 symbols+resolve — DONE
- Plan 4 passes+diagnostics — DONE
- Plan 5a workspace+reindex (§9) — DONE+reviewed (race fix applied)
- Plan 5b stdlib/cache (§10) — doc just written, NOT executed
- Remaining: 5c deps/manifest (§11), 6 lsp (§12), 7 repl (§13)
User-chosen sequence: 5b → 5c → 6 → 7.
IMMEDIATE NEXT ACTION
Plan 5b doc COMPLETE at docs/superpowers/plans/2026-07-25-plan-05b-stdlib-cache.md (1201 lines, all 7 tasks + Self-Review filled, no real placeholders — the one <FILL/TBD grep hit is inside Self-Review prose). Todo Plan 5b self-review is in_progress.
1. Finish Plan 5b self-review (already effectively done inline — mark complete).
2. EXECUTE Plan 5b via subagent-driven-development (controller-direct for well-specified tasks: TDD + go build + go test -count=1 + go vet + gofmt + commit + inline verify). New pkg internal/core/libs/. Tasks in order:
- T1 source.go — Source{List()/Read()}, embed.FS default + SYSML_LIBRARY_PATH dirSource override + stdlib/ScalarValues.kerml placeholder
- T2 curated payload stdlib/ScalarValues.kerml (namespace-core only, parses 0 diags)
- T3 record.go IndexRecord/symRecord gob + adds Scope.Members()/memberOrder to symbols pkg
- T4 cache.go Cache keyed sha256(content)+"-v"+formatVersion at $XDG_CACHE_HOME/sysml-ls/libs/
- T5 loader.go lazy parse→AddDocument
- T6 cache integration + adds symbols.RecordEntry + Index.AddRecords(name,entries) (AST-less symbols, removable via existing contributions map)
- T7 integration tests
3. After all 7: final whole-impl reviewer subagent (base 8ee50ee..HEAD, VERIFY claims vs disk before complying), then finishing-a-development-branch (precedent: commits live on master, merge/keep = NO-OP), then Plan 5c.
Standing rules
- Design/plan docs: OUTLINE-FIRST-THEN-FILL (skeleton then edit each section).
- ONE PLAN AT A TIME, just-in-time.
- ALWAYS verify reviewer/plan grammar+code claims vs pilot grammars + current delivered types before complying.
- AGENTS.md caveman-terse chat (code/commits/PRs normal).
Gotchas
- Real OMG stdlib files → many ErrorNodes (no def/usage taxonomy yet) → use curated namespace-core payload, real files drop in later.
- Zero-value passes.Severity == SeverityError (set explicitly for non-error fixtures).
- T{}.Method() needs parens: (T{}).Method().
- First go test slow → go build first; internal pkgs untestable from /tmp.
- symbols.Symbol.Decl (ast.Node) + Scope/OwnerScope pointers NOT gob-friendly → persist reduced records.
Key delivered APIs (Plan 1–5a, disk-verified)
- model.NewWorkspace() + Open/Update/SetOnDisk/Close/Remove; LookupQualified(fqn)[]*symbols.Symbol (RLock+copied; Index() accessor REMOVED); Document(name); Diagnostics(name)[]passes.Diagnostic. EventLoop/Debouncer/Watcher.
- symbols: NewIndex/AddDocument/RemoveDocument(per-symbol contributions)/LookupQualified/DocumentRoot; Build(root); Symbol{Name,Kind,Decl,Visibility,DeclSpan,Scopenil for leaves,OwnerScope}; SymbolKind enum. Scope: NewScope/Parent/Node/Children/AddChild/Define/LookupLocal/LookupLocalAll (members map + memberOrder unexported).
- parser.New(sf).ParseFile()*ast.RootNamespace; parser.Parser.Diagnostics []Diagnostic{Span,Message}.
- resolve.New(idx); ResolveDocument/ResolveQualified/ResolveName/ResolveAliasTarget.
- passes.Analyze(name,root,parseDiags,idx)[]Diagnostic; Diagnostic{Severity,Span,Message,Code,Source}.
- source.New(name,[]byte); .Text(Span)/.Lines().PosAt(offset)/.Bytes(); Span{Offset,Len}+End().
- ast.Node interface: getters ONLY Span/LeadingTrivia/TrailingTrivia (setters on *NodeBase).
Commit chain (HEAD 8ee50ee)
...e752bf3(P4 review) → 82a8f72(P5a-T1) → e075044 → 12f3a05 → 3989e24 → 8f8c72d → 0a7975c → ae30335 → 5fa0352(P5a-T8) → 8ee50ee(P5a review fix). go.mod: github.com/fsnotify/fsnotify v1.10.1 (direct) + golang.org/x/sys (indirect).
Full history in compressed blocks (b51)–(b95).
Now compressing to reclaim context.
⚙compress
SysML-v2-Go — Handoff Notes
Project
New independent SysML v2 (incl KerML) language implementation in Go → goal: LSP server + CLI REPL. Module github.com/Open-MBEE/Systemica, root /home/han/IdeaProjects/Systems-Modeling, IS a git repo on master (env banner may lie). Go 1.25. Pilot reference clone ./SysML-v2-Pilot-Implementation/ (gitignored, nested .git — NEVER commit); stdlib there at sysml.library/; grammars org.omg.kerml.xtext, org.omg.kerml.expressions.xtext, org.omg.sysml.xtext.
Status (HEAD 8ee50ee on master)
7-plan decomposition:
- Plan 1 source+lexer — DONE
- Plan 2 ast+parser — DONE (namespace-core + full expressions; NO def/usage taxonomy: datatype/specializes/part/feature etc NOT parsed → produce ErrorNodes)
- Plan 3 symbols+resolve — DONE
- Plan 4 passes+diagnostics — DONE
- Plan 5a workspace/reindex (§9) — DONE + reviewed
- Plan 5b stdlib/cache (§10) — plan doc WRITTEN, execution NOT started ← YOU ARE HERE
- Plan 5c deps/manifest (§11), Plan 6 lsp (§12), Plan 7 repl (§13) — pending
Commit chain tail: ...e752bf3(P4 review) → 82a8f72 → e075044 → 12f3a05 → 3989e24 → 8f8c72d → 0a7975c → ae30335 → 5fa0352 → 8ee50ee(P5a review fix). go.mod: github.com/fsnotify/fsnotify v1.10.1 (direct) + golang.org/x/sys (indirect).
Immediate next action
Execute Plan 5b via subagent-driven-development. Plan doc docs/superpowers/plans/2026-07-25-plan-05b-stdlib-cache.md is COMPLETE (1201 lines, all 7 tasks + Self-Review, no real placeholders — the one <FILL>/TBD grep hit is inside a Self-Review sentence). New pkg internal/core/libs/.
Plan 5b tasks (each: failing test → run-fail → impl → run-pass → gofmt+vet+commit; controller-direct OK, well-specified):
1. Source abstraction — source.go: Source{List()[]string; Read(name)([]byte,error)}; DefaultSource() = embedSource (//go:embed stdlib/*.kerml) unless SYSML_LIBRARY_PATH set → dirSource. Curated payload stdlib/ScalarValues.kerml placeholder.
2. Curated payload — real stdlib/ScalarValues.kerml = standard library package ScalarValues { doc /* */ namespace Boolean; ... namespace Positive; } (namespace-core only, parses 0 diagnostics; real OMG file deferred — it produces ~11 ErrorNodes today).
3. IndexRecord — record.go: IndexRecord{Name; Symbols []symRecord}, symRecord{FQN string; Kind symbols.SymbolKind; Span source.Span; Supers []string}; recordFromIndex(name,idx) walks idx.DocumentRoot(name) via Scope.Children(). Adds symbols.Scope.Members() []*Symbol + memberOrder field (deviation — enumerate via public API).
4. Cache — cache.go: Cache{dir}, NewCache() = $XDG_CACHE_HOME|os.UserCacheDir + /sysml-ls/libs/; keyFor(content) = hex(sha256)+"-v"+formatVersion; Load/Store gob <key>.idx.
5. Loader — loader.go: Loader{src,cache}, Load(name,idx) = Read→parse→idx.AddDocument (no cache yet).
6. Cache integration — Load checks cache.keyFor(content): hit→idx.AddRecords(name, recordEntries(rec)) skip parse; miss→parse+AddDocument+recordFromIndex+Store. Adds symbols.RecordEntry{FQN,Kind,Span} + Index.AddRecords(name,[]RecordEntry) (deviation — AST-less registration into idx.fqn+idx.contributions; verify those private field names in index.go before writing).
7. Integration tests — embed end-to-end, SYSML_LIBRARY_PATH override (testdata/customlib/Custom.kerml), cache stale-content-reparsed.
After all 7: dispatch final whole-impl code reviewer subagent (base 8ee50ee..HEAD), verify claims vs disk before complying, fix loop. Then finishing-a-development-branch skill (precedent: work lives directly on master, user chose merge/keep = NO-OP). Then Plan 5c → 6 → 7. Sequence fixed by user: 5b, 5c, then Plan 6.
Delivered APIs (Plan 1–5a, disk-verified)
- parser.New(sf *source.SourceFile)*Parser; (*Parser).ParseFile()*ast.RootNamespace; Parser.Diagnostics []parser.Diagnostic{Span; Message}
- symbols.NewIndex()*Index; AddDocument(name,root)/RemoveDocument(name)per-symbol contributions map/LookupQualified(fqn)[]*Symbol/DocumentRoot(name)*Scope; NewIndexFromDoc; Build(root)*Scope. Symbol{Name;Kind;Decl ast.Node;Visibility;DeclSpan source.Span;Scope *Scope[child,nil for leaves];OwnerScope *Scope}. SymbolKind{SymbolUnknown,SymbolPackage,SymbolNamespace,SymbolAlias,SymbolDependency,SymbolComment,SymbolDocumentation,SymbolTextualRepresentation}+String(). Scope.Children()/Parent()/Node()/AddChild/Define(name,sym)/LookupLocal/LookupLocalAll (members map is UNEXPORTED — hence Task 3 adds Members()).
- resolve.New(idx)*Resolver; ResolveDocument(name,root) APPENDS Resolver.Diagnostics []resolve.Diagnostic{Span;Message}; ResolveQualified/ResolveName/ResolveAliasTarget
- passes.Analyze(name,root,parseDiags []passes.Diagnostic,idx)[]passes.Diagnostic; passes.Diagnostic{Severity;Span;Message;Code;Source}; Severity{SeverityError=iota=zero-value,...}+String()
- model.NewWorkspace()*Workspace: Open/Update/SetOnDisk/Close/Remove; LookupQualified(fqn)[]*symbols.Symbol (RLock+copied; Index() accessor REMOVED); Document(name)*Document; Diagnostics(name)[]passes.Diagnostic. Plus EventLoop/Debouncer/Watcher.
- source.New(name,[]byte)*SourceFile; .Text(Span)/.Bytes()/.Name()/.Lines().PosAt(offset)Pos{Line,Col} 1-based; Span{Offset,Len}+End()
- ast.Node interface = getters ONLY Span()/LeadingTrivia()/TrailingTrivia(); setters on *NodeBase only.
Standing rules & gotchas
- Write design/plan docs OUTLINE-FIRST-THEN-FILL; ONE PLAN AT A TIME just-in-time.
- AGENTS.md caveman-terse chat mode ACTIVE (fragments, drop filler; code/commits/PRs normal).
- Verify reviewer/plan grammar+code claims vs pilot grammars + current delivered types before complying.
- zero-value passes.Severity == SeverityError (set explicitly for non-error fixtures).
- (T{}).Method() needs parens in Go statement/if position.
- First go test slow → go build ./internal/core/... first, then go test ... -count=1 -timeout 120-180s > /tmp/opencode/x.log 2>&1; tail. Internal pkgs UNTESTABLE from /tmp. Use -race for concurrency.
- docs/ dir left UNTRACKED (matches Plans 1–5a; user never answered commit-docs question).
- Spec docs/superpowers/specs/2026-07-25-sysml-v2-go-design.md (15 §, authoritative). §10 exact text in block (b91); §11 (Plan 5c) in (b91).
Warning
Session is at MAX CONTEXT. The environment has been rejecting compress calls (Tool execution aborted) repeatedly. Next agent should compress older resolved history early and keep the compressed-block references (b51–b95) intact.
