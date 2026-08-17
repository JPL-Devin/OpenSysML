package export

// ExperimentalNotice is the wording every surface reports the RDF mapping's
// status in, so the CLI, the REPL, the service and the docs agree.
const ExperimentalNotice = "RDF conversion is experimental: the mapping covers model structure only, " +
	"refuses a model whose bodies state behavior, and its vocabulary may change without a " +
	"compatibility path; see docs/reference/rdf-mapping.md § Status"

// IsExperimental reports whether a conversion between these formats goes
// through the RDF mapping. Notation to notation does not.
func IsExperimental(from, to Format) bool {
	return from == FormatTurtle || to == FormatTurtle
}
