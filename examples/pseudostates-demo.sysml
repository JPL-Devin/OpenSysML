package PseudostatesDemo {
    private import ScalarValues::*;
    
    // Demonstrates choice and junction pseudostates for conditional branching.
    // These pseudostates are OpenSysML extensions with no SysML v2 spelling.
    
    // Example 1: Choice (Dynamic Branching)
    // Guards evaluated at runtime when choice is entered
    state ChoiceExample {
        entry; then idle;
        state idle;
        choice evaluate;
        state pathA;
        state pathB;
        state pathC;
        
        transition first idle then evaluate;
        transition first evaluate then pathA;
        transition first evaluate then pathB;
        transition first evaluate then pathC;
    }
    
    // Example 2: Junction (Static Branching)
    // Guards pre-evaluated before junction is entered
    state JunctionExample {
        entry; then receiving;
        state receiving;
        junction route;
        state highPriority;
        state normalPriority;
        state lowPriority;
        
        transition first receiving then route;
        transition first route then highPriority;
        transition first route then normalPriority;
        transition first route then lowPriority;
    }
    
    // Key difference:
    // Choice: dynamic branching (guards evaluated when entered)
    // Junction: static branching (guards pre-evaluated)
}
