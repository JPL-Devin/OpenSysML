package JunctionTest {
    state WorkflowControl {
        attribute status : String = "unknown";
        attribute priority : Integer = 2;
        
        initial init;
        state start;
        junction routeDecision;
        state highPriority;
        state normalPriority;
        final done;
        
        succession first init then start;
        transition start to routeDecision;
        transition routeDecision to highPriority if priority > 5;
        transition routeDecision to normalPriority;  // else branch
        
        entry highPriority { assign status := "high"; }
        entry normalPriority { assign status := "normal"; }
        
        transition highPriority to done;
        transition normalPriority to done;
    }
}
