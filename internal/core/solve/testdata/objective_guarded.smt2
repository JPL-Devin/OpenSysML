; OpenSysML SMT-LIB2 translation of analysis GuardedRatio
; the runtime evaluator remains normative; solving is an optional extension
; objectives are optimized lexicographically, in the order the analysis declares them:
; each is optimized within what the ones before it already settled
(set-option :opt.priority lex)
(set-logic QF_NIA)
; test::GuardedRatio::parts, declared at objectives.sysml:133:3
(declare-const |test::GuardedRatio::parts| Int)
; test::GuardedRatio::total, declared at objectives.sysml:132:3
(declare-const |test::GuardedRatio::total| Int)
; well-definedness: total / parts >= 2 — analysis GuardedRatio
(assert (distinct |test::GuardedRatio::parts| 0))
; required condition: total / parts >= 2 — analysis GuardedRatio
(assert (>= (ite (>= |test::GuardedRatio::total| 0) (div |test::GuardedRatio::total| |test::GuardedRatio::parts|) (- (div (- |test::GuardedRatio::total|) |test::GuardedRatio::parts|))) 2))
; required condition: parts >= 1 — analysis GuardedRatio
(assert (>= |test::GuardedRatio::parts| 1))
; required condition: parts <= 4 — analysis GuardedRatio
(assert (<= |test::GuardedRatio::parts| 4))
; required condition: total <= 40 — analysis GuardedRatio
(assert (<= |test::GuardedRatio::total| 40))
; maximize: mostParts = parts, at objectives.sysml:147:25
(maximize |test::GuardedRatio::parts|)
(check-sat)
