package solve

import (
	"math/big"
	"strings"
)

// assign renders one variable's solver value in the notation's own terms, keeping
// the solver's S-expression for a value the notation cannot write.
func assign(v *Var, value sexpr) Assignment {
	raw := value.String()
	text, ok := renderValue(v, value)
	if !ok {
		return Assignment{Var: v, Value: raw, Raw: raw}
	}
	return Assignment{Var: v, Value: text, Raw: raw, Rendered: true}
}

// renderValue writes a value as the notation does: a quantity with its unit, an
// enumeration or variant by name, a number, a boolean or a string.
func renderValue(v *Var, value sexpr) (string, bool) {
	switch v.Sort.Kind {
	case SortBool:
		if !value.IsList && (value.Atom == "true" || value.Atom == "false") {
			return value.Atom, true
		}
	case SortString:
		if !value.IsList && value.Quoted {
			return `"` + value.Atom + `"`, true
		}
	case SortDatatype:
		if !value.IsList {
			name := smtName(value.Atom)
			for _, candidate := range v.Sort.Values {
				if candidate == name {
					return name, true
				}
			}
		}
	case SortInt:
		if rat, ok := ratOfSexpr(value); ok && rat.IsInt() {
			return rat.Num().String(), true
		}
	case SortReal:
		if rat, ok := ratOfSexpr(value); ok {
			return withUnit(renderRat(rat), v.Dimension), true
		}
	}
	return "", false
}

// withUnit writes a magnitude with the base unit it is expressed in, as a
// quantity is written, and bare when it has none.
func withUnit(magnitude, dimension string) string {
	if dimension == "" {
		return magnitude
	}
	return magnitude + " [" + dimension + "]"
}

// renderRat writes an exact rational as the notation writes a number: a decimal
// when it has a terminating one, else the quotient it is.
func renderRat(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String() + ".0"
	}
	if digits, ok := decimalDigits(r.Denom()); ok {
		return r.FloatString(digits)
	}
	return r.Num().String() + "/" + r.Denom().String()
}

// ratOfSexpr reads a numeral, a decimal, or the negation, quotient or widening a
// solver writes a numeric model value with.
func ratOfSexpr(value sexpr) (*big.Rat, bool) {
	if !value.IsList {
		if value.Quoted {
			return nil, false
		}
		rat, ok := new(big.Rat).SetString(value.Atom)
		if !ok || strings.ContainsAny(value.Atom, "eE/") {
			// SetString accepts exponents and quotients, which SMT-LIB numerals
			// and decimals are not.
			return nil, false
		}
		return rat, true
	}
	switch {
	case len(value.List) == 2 && value.List[0].Atom == "-":
		inner, ok := ratOfSexpr(value.List[1])
		if !ok {
			return nil, false
		}
		return inner.Neg(inner), true
	case len(value.List) == 2 && value.List[0].Atom == "to_real":
		return ratOfSexpr(value.List[1])
	case len(value.List) == 3 && value.List[0].Atom == "/":
		num, ok := ratOfSexpr(value.List[1])
		if !ok {
			return nil, false
		}
		den, okDen := ratOfSexpr(value.List[2])
		if !okDen || den.Sign() == 0 {
			return nil, false
		}
		return num.Quo(num, den), true
	}
	return nil, false
}
