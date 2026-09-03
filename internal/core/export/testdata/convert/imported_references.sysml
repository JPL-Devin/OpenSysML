package OtherPkg {
    item def BudgetLedger;
    enum def Tempo {
        operative;
        planning;
    }
    package Inner {
        part def Wheel;
        part wheels : Wheel;
    }
}
package R90 {
    public import OtherPkg::*;
    alias Ledger for OtherPkg::BudgetLedger;
    part def A {
        item custodyBudget : OtherPkg::BudgetLedger;
        attribute t : Tempo = Tempo::operative;
        item b2 : BudgetLedger;
        item b3 : Ledger;
        part w : Inner::Wheel :> Inner::wheels;
        part unresolved : Elsewhere::Missing;
    }
    part def B :> A {
        part :>> w : Inner::Wheel;
    }
}
