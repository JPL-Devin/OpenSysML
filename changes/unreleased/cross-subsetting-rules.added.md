- **A `crosses` clause is validated against the whole of KerML's cross-subsetting rules.** A
  cross subsetting is now an error unless its owner is an end feature of a type that declares
  two or more ends (`Cross subsetting must be owned by one of two or more end features`); the
  crossed feature must be a two-step feature chain that, on a binary association or connection,
  starts at the other end (`Cross subsetting must chain through an opposite end feature`), so
  `end a : A crosses b;` and `end a : A crosses a.x;` are reported where before only a chain
  through a non-end was; a feature may cross at most once (`At most one cross subsetting is
  allowed`, reported on every clause after the first, as the reference implementation's source
  intends where its pinned build only crashes); and an end that redefines another end — by a
  `redefines` clause or by its position in an association that specializes another — must
  cross a feature that specializes what the redefined end crosses (`Cross feature must
  specialized redefined-end cross features`). The rules read the same for KerML `assoc` and
  `connector` ends and for SysML `connection def` and `connection` ends. A cross feature an
  end declares in its own body (`end a : A { member feature x : B; }`) or inline ahead of
  itself (`end x [0..1] feature a : A;`) now implicitly subsets the cross feature of each end
  its owner redefines, as the specification's implied specializations require, so the n-ary
  association examples of the reference stay silent; the inline cross feature is a member of
  its end, so `A::a::x` resolves to it.
