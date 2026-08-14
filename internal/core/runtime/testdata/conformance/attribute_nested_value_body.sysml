// A nested value body supplies a struct-typed attribute's features, through a
// declaration, a plain redefinition, a renaming one, and a body nested two
// levels deep; a renaming redefinition takes the type it does not restate.
package test {
    private import ScalarValues::Real;

    attribute def Cost { attribute v : Real = 1.0; attribute w : Real = 2.0; }
    attribute def Material :> Cost;
    attribute def Assembly { attribute piece : Cost; }

    part def Ring {
        attribute material : Material;
        attribute ringCost : Cost;
        attribute assembly : Assembly;
    }

    part def Band :> Ring { attribute bandCost :>> ringCost; }

    part b : Band {
        attribute :>> material { attribute :>> v = 3.0; }
        attribute :>> bandCost { attribute :>> v = 400.0; }
        attribute :>> assembly { attribute :>> piece { attribute :>> v = 5.0; } }
        attribute declared : Cost { attribute :>> v = 7.0; }

        assert constraint plainRedefinition { material.v == 3.0 }
        assert constraint renamingRedefinition { bandCost.v == 400.0 }
        assert constraint baseNameReadsTheSame { ringCost.v == 400.0 }
        assert constraint twoLevelsDeep { assembly.piece.v == 5.0 }
        assert constraint declarationWithBody { declared.v == 7.0 }
        assert constraint keepsInheritedDefault { material.w == 2.0 }
    }

    part untouched : Band;
}
