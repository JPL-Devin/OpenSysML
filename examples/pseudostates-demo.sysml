package PseudostatesDemo {
    private import ScalarValues::*;
    
    // Demonstrates choice and junction pseudostates for conditional branching
    
    // Example 1: Choice (Dynamic Branching)
    // Guards evaluated at runtime when choice is entered
    state ChoiceExample {
        initial start;
        state idle;
        choice evaluate;
        state pathA;
        state pathB;
        state pathC;
        
        start then idle;
        transition idle to evaluate;
        transition evaluate to pathA;
        transition evaluate to pathB;
        transition evaluate to pathC;
    }
    
    // Example 2: Junction (Static Branching)
    // Guards pre-evaluated before junction is entered
    state JunctionExample {
        initial start;
        state receiving;
        junction route;
        state highPriority;
        state normalPriority;
        state lowPriority;
        
        start then receiving;
        transition receiving to route;
        transition route to highPriority;
        transition route to normalPriority;
        transition route to lowPriority;
    }
    
    // Key difference:
    // Choice: dynamic branching (guards evaluated when entered)
    // Junction: static branching (guards pre-evaluated)
}
