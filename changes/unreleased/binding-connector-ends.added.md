- **A binding connector's ends are connector ends, so they may be named.** `bind e1 ::> a =
  e2 references b;` and KerML `binding of e1 ::> a = e2 references b;` declare two end features
  `e1` and `e2` owned by the binding, each reference-subsetting the feature it binds, exactly as
  `succession first s1 ::> a then s2 ::> b;` and `connect c1 ::> a to c2 ::> b;` already did.
  `e1` was read as the binding's own name and `bind e3 ::> a = b;` dropped `e3` on the floor;
  both are now end names that hover, go-to-definition and rename find, and the semantic binding
  still joins the two referenced features. The RDF graph states both ends as `sysx:relatedFeature`
  end nodes carrying `sysx:endName`, `sysx:endIndex` and each end's multiplicity, instead of
  putting the second end in `sysml:value`; a graph stripped of its source text writes back to the
  same notation, spelling a named end `::>` unless `sysx:endReferencesKeyword` records the word
  `references`. An end with two names, a named end with no referenced feature, or a binding with
  fewer than two end nodes is refused by name rather than failing as a notation error.
