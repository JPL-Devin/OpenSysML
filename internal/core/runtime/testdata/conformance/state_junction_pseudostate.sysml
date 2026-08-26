package JunctionTest {
    state WorkflowControl {
        attribute status : String = "unknown";
        attribute priority : Integer = 2;
        
        entry; then init;
        state init;
        state start;
        junction routeDecision;
        state highPriority;
        state normalPriority;
        final done;
        
        succession first init then start;
        transition first start then routeDecision;
        transition first routeDecision if priority > 5 then highPriority;
        transition first routeDecision then normalPriority;  // else branch
        
        entry highPriority { assign status := "high"; }
        entry normalPriority { assign status := "normal"; }
        
        transition first highPriority then done;
        transition first normalPriority then done;
    }
}
