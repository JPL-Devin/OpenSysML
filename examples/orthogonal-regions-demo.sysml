package OrthogonalRegionsDemo {
    import ScalarValues::*;
    
    // Demonstrates concurrent orthogonal regions in state machines
    // Each region executes independently and processes events in parallel
    
    // Example: Traffic Light with Pedestrian Crossing
    state def TrafficLight {
        // Vehicle signal region
        region vehicleSignal {
            initial start;
            state red;
            state yellow;
            state green;
            
            then start red;
            transition red to yellow;
            transition yellow to green;
            transition green to red;
        }
        
        // Pedestrian signal region (runs concurrently)
        region pedestrianSignal {
            initial start;
            state dontWalk;
            state walk;
            
            then start dontWalk;
            transition dontWalk to walk;
            transition walk to dontWalk;
        }
        
        // Key concepts:
        // - Both regions start simultaneously (v_start→red, p_start→dontWalk)
        // - Active configuration: (red, dontWalk) or (yellow, walk) etc
        // - Events broadcast to both regions independently
        // - Run-to-completion: one transition per region per event
    }
}
