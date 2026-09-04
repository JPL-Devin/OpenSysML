package runtime

// registerUnevaluableDeclarations registers, by name and declared signature,
// every library function declaration the runtime cannot evaluate, so a call
// reports the reason.
func registerUnevaluableDeclarations() {
	typeArgument := "the function form takes a type as a value, which the runtime evaluates only in the operator notation `x istype T`"
	for _, op := range []string{"istype", "hastype", "@", "@@"} {
		registerUnevaluable("BaseFunctions::"+op, []declaredParam{optionalParam("seq"), param("type")}, typeArgument)
	}
	registerUnevaluable("BaseFunctions::all", nil,
		"'all' needs the extent of a type, which the runtime does not enumerate")
	registerUnevaluable("BaseFunctions::as", []declaredParam{optionalParam("seq")},
		"a cast needs the runtime type of a value, which values do not carry yet")
	registerUnevaluable("BaseFunctions::meta", []declaredParam{optionalParam("seq")},
		"metadata access is evaluated from a MetadataAccessExpression, not this function")
	registerUnevaluable("BaseFunctions::[", []declaredParam{optionalParam("x"), optionalParam("y")},
		"the runtime has no Array value kind to index into")
	registerUnevaluable("CollectionFunctions::array#", []declaredParam{param("arr"), param("indexes")},
		"the runtime has no Array value kind to index into")
	registerUnevaluable("ControlFunctions::.", []declaredParam{optionalParam("source")},
		"a feature chain is evaluated from a FeatureChainExpression, not this function")

	noBitwise := "bitwise complement is declared by no function library the runtime applies"
	registerUnevaluable("DataFunctions::~", []declaredParam{param("x")}, noBitwise)
	registerUnevaluable("ScalarFunctions::~", []declaredParam{param("x")}, noBitwise)
}
