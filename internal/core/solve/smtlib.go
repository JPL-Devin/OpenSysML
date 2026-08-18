package solve

import (
	"fmt"
	"io"
	"math/big"
	"strings"
)

// Script renders the query as an SMT-LIB2 script: its logic, declarations,
// assertions each preceded by a comment naming where it came from, and
// `check-sat`. Output is deterministic, so it compares byte for byte.
func Script(q *Query) string {
	var b strings.Builder
	writeScript(&b, q)
	return b.String()
}

// Write writes the query's script to w.
func Write(w io.Writer, q *Query) error {
	_, err := io.WriteString(w, Script(q))
	return err
}

// writeScript writes the script itself; Script and Write share it.
func writeScript(b *strings.Builder, q *Query) {
	fmt.Fprintf(b, "; OpenSysML SMT-LIB2 translation of %s %s\n", q.Kind, comment(q.Element))
	b.WriteString("; the runtime evaluator remains normative; solving is an optional extension\n")
	if q.Negated {
		b.WriteString("; the element asserts that its required conditions do not all hold\n")
	}
	fmt.Fprintf(b, "(set-logic %s)\n", q.Logic())

	for _, s := range q.Sorts {
		values := make([]string, 0, len(s.Values))
		for _, v := range s.Values {
			values = append(values, "("+smtSymbol(v)+")")
		}
		fmt.Fprintf(b, "; %s of %s\n", s.String(), comment(s.Origin))
		fmt.Fprintf(b, "(declare-datatypes ((%s 0)) ((%s)))\n", s.String(), strings.Join(values, " "))
	}

	for _, v := range q.Vars {
		b.WriteString("; " + varComment(v) + "\n")
		fmt.Fprintf(b, "(declare-const %s %s)\n", smtSymbol(v.Name), v.Sort.String())
	}

	for _, a := range q.Assertions {
		b.WriteString("; " + assertionComment(a.From) + "\n")
		fmt.Fprintf(b, "(assert %s)\n", writeTerm(a.Term))
	}

	b.WriteString("(check-sat)\n")
}

// varComment describes a declared variable: the feature it stands for, the
// dimension its magnitude is expressed in, and where it was declared.
func varComment(v *Var) string {
	out := comment(v.Name)
	if v.Dimension != "" {
		out += " in base units of " + comment(v.Dimension)
	}
	if v.Location != "" {
		out += ", declared at " + comment(v.Location)
	}
	return out
}

// assertionComment names what an assertion came from: its role, the condition as
// written, the element stating it, and where it was written.
func assertionComment(p Provenance) string {
	out := p.Role.String() + ": " + comment(p.Condition)
	out += fmt.Sprintf(" — %s %s", p.Kind, comment(p.Element))
	if p.Declared != nil && p.Declared.Name != "" && p.Declared.Name != p.Element {
		out += ", declared by " + comment(p.Declared.Name)
	}
	if p.Location != "" {
		out += ", at " + comment(p.Location)
	}
	return out
}

// comment renders text safely on one comment line.
func comment(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// writeTerm renders a term as an S-expression.
func writeTerm(t *Term) string {
	switch t.Op {
	case OpBool:
		if t.Bool {
			return "true"
		}
		return "false"
	case OpInt:
		return writeInt(t.Int)
	case OpReal:
		return writeRat(t.Real)
	case OpString:
		return `"` + strings.ReplaceAll(t.Str, `"`, `""`) + `"`
	case OpValue:
		return smtSymbol(t.Str)
	case OpVar:
		return smtSymbol(t.Var.Name)
	}
	args := make([]string, 0, len(t.Args))
	for _, arg := range t.Args {
		args = append(args, writeTerm(arg))
	}
	op := smtOps[t.Op]
	if t.Op == OpNe {
		// `distinct` is SMT-LIB's inequality; the notation writes `!=`.
		op = "distinct"
	}
	return "(" + op + " " + strings.Join(args, " ") + ")"
}

// writeInt renders an integer literal, as SMT-LIB numerals are non-negative.
func writeInt(i int64) string {
	if i < 0 {
		return fmt.Sprintf("(- %d)", -i)
	}
	return fmt.Sprintf("%d", i)
}

// writeRat renders a real literal exactly: as a decimal when the magnitude has a
// terminating one, else as the quotient it is, so no rounding enters the script.
func writeRat(r *big.Rat) string {
	if r.Sign() < 0 {
		return "(- " + writeRat(new(big.Rat).Neg(r)) + ")"
	}
	if digits, ok := decimalDigits(r.Denom()); ok {
		return r.FloatString(digits)
	}
	return fmt.Sprintf("(/ %s.0 %s.0)", r.Num().String(), r.Denom().String())
}

// decimalDigits returns how many fractional digits write a magnitude of this
// denominator exactly, which only a denominator of 2s and 5s has.
func decimalDigits(den *big.Int) (int, bool) {
	rest, digits := new(big.Int).Set(den), 0
	for _, prime := range []*big.Int{big.NewInt(2), big.NewInt(5)} {
		count := 0
		quo, rem := new(big.Int), new(big.Int)
		for {
			quo.QuoRem(rest, prime, rem)
			if rem.Sign() != 0 {
				break
			}
			rest.Set(quo)
			count++
		}
		digits = max(digits, count)
	}
	if rest.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}
	// A decimal literal in SMT-LIB2 always writes a fractional digit.
	return max(digits, 1), true
}
