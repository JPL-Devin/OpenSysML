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
        attribute t : OtherPkg::Tempo = OtherPkg::Tempo::operative;
        item b2 : OtherPkg::BudgetLedger;
        item b3 : OtherPkg::BudgetLedger;
        part w : OtherPkg::Inner::Wheel subsets OtherPkg::Inner::wheels;
        part unresolved : Elsewhere::Missing;
    }
    part def B specializes A {
        part : OtherPkg::Inner::Wheel redefines w;
    }
}
