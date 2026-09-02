package runtime

// registerUnevaluableDeclarations registers, by name, every library function
// declaration the runtime cannot evaluate, so a call reports the reason.
func registerUnevaluableDeclarations() {
	typeArgument := "the function form takes a type as a value, which the runtime evaluates only in the operator notation `x istype T`"
	for _, op := range []string{"istype", "hastype", "@", "@@"} {
		registerUnevaluable("BaseFunctions::"+op, []string{"seq", "type"}, 2, typeArgument)
	}
	registerUnevaluable("BaseFunctions::all", nil, 0,
		"'all' needs the extent of a type, which the runtime does not enumerate")
	registerUnevaluable("BaseFunctions::as", []string{"seq"}, 0,
		"a cast needs the runtime type of a value, which values do not carry yet")
	registerUnevaluable("BaseFunctions::meta", []string{"seq"}, 0,
		"metadata access is evaluated from a MetadataAccessExpression, not this function")
	registerUnevaluable("BaseFunctions::[", []string{"x", "y"}, 0,
		"the runtime has no Array value kind to index into")
	registerUnevaluable("CollectionFunctions::array#", []string{"arr", "indexes"}, 2,
		"the runtime has no Array value kind to index into")
	registerUnevaluable("ControlFunctions::.", []string{"source"}, 0,
		"a feature chain is evaluated from a FeatureChainExpression, not this function")

	noBitwise := "bitwise complement is declared by no function library the runtime applies"
	registerUnevaluable("DataFunctions::~", []string{"x"}, 1, noBitwise)
	registerUnevaluable("ScalarFunctions::~", []string{"x"}, 1, noBitwise)

	noOccurrenceTime := "occurrence-time and lifecycle semantics are not represented by the evaluator"
	registerUnevaluable("OccurrenceFunctions::===", []string{"x", "y"}, 0, noOccurrenceTime)
	registerUnevaluable("OccurrenceFunctions::isDuring", []string{"occ"}, 1, noOccurrenceTime)
	registerUnevaluable("OccurrenceFunctions::create", []string{"occ"}, 1, noOccurrenceTime)
	registerUnevaluable("OccurrenceFunctions::destroy", []string{"occ"}, 0, noOccurrenceTime)
	registerUnevaluable("OccurrenceFunctions::addNew", []string{"group", "occ"}, 0, noOccurrenceTime)
	registerUnevaluable("OccurrenceFunctions::addNewAt", []string{"group", "occ", "index"}, 0, noOccurrenceTime)
}
