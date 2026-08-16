package repl

// InstantiationReport is what creating an object produced: the lines a caller
// prints, and the diagnostics materializing the object's slots reported.
type InstantiationReport struct {
	Lines []string
	// SlotErrors are the slots that could not be materialized, in the order they
	// were read: a default whose value count does not conform to the feature's
	// multiplicity is one.
	SlotErrors []string
}

// InstantiateReport creates the named object and materializes its slots, so a
// non-interactive caller reports what materialization found rather than leaving
// it to whoever reads a slot next. An object that cannot be created at all is an
// error; a slot that cannot be materialized is a finding about the model.
func (s *Session) InstantiateReport(name string) (InstantiationReport, error) {
	lines, err := s.InstantiateNamed(name)
	if err != nil {
		return InstantiationReport{}, err
	}
	report := InstantiationReport{Lines: lines}

	_, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		// Unreachable: the object was just created under this name.
		return report, nil
	}
	inst, ok := s.instances[fqn]
	if !ok {
		return report, nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return report, err
	}
	for _, slotErr := range ctx.MaterializationErrors(inst) {
		report.SlotErrors = append(report.SlotErrors, slotErr.Error())
	}
	return report, nil
}
