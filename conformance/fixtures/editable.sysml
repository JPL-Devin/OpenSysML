package Demo {
    // The mass of one unit, measured on the bench.
    part def SC {
        attribute unitMass : ISQ::MassValue = 1000.0[SI::kg];

        attribute margin : ISQ::MassValue;
    }

    part sc : SC;
}
