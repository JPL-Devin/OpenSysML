// A redefinition renames one feature (KerML 1.0 §7.3.4.5), so a value written
// under either name is the value both names read, however many names the chain
// of redefinitions gives it and whichever declaration wrote it.
package test {
    private import ScalarValues::Real;

    part def Ring { attribute ringCost : Real; attribute color : Real = 1.0; }
    part def Band :> Ring { attribute bandCost :>> ringCost; }
    part def Deluxe :> Band { attribute deluxeCost :>> bandCost; }

    part onRedefining : Band { attribute :>> bandCost = 400.0; }
    part onBase : Band { attribute :>> ringCost = 500.0; }
    part renamedTwice : Deluxe { attribute :>> ringCost = 700.0; }

    // The value is written in an abstract part usage the configured one
    // specializes, so it reaches the redefinition through that usage.
    abstract part family { part band : Band { attribute :>> bandCost = 400.0; } }
    part configured :> family { part :>> band { attribute :>> color = 2.0; } }

    part reader {
        attribute redefiningName : Real = onRedefining.bandCost;
        attribute redefiningBase : Real = onRedefining.ringCost;
        attribute baseName : Real = onBase.ringCost;
        attribute baseRedefining : Real = onBase.bandCost;
        attribute deep : Real = renamedTwice.deluxeCost + renamedTwice.bandCost + renamedTwice.ringCost;
        attribute throughFamily : Real = configured.band.ringCost;
    }
}
