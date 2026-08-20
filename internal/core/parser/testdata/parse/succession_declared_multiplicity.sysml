// A succession states a declaration of its own, so a multiplicity written
// after a type or a redefinition is the declaration's, not the first end's
// (SysML.xtext:1033 SuccessionAsUsage, UsageDeclaration).
package SuccessionDeclarations {
    connection def Before;

    action def Process {
        action paint;
        action dry;

        succession named : Before [1] first paint then dry;
        succession redefines named : Before [1] first paint then dry;
        succession [1] first paint then dry;
        succession first paint then dry;
    }
}
