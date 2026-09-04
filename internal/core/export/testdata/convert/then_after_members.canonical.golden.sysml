package ThenAfterMembers {
    item def Signal;
    action def Step {
        in item x : Signal;
        out item y : Signal;
    }
    action def AfterFlow {
        action a : Step;
        flow from a.y to b.x;
        then action b : Step;
    }
    action def AfterBind {
        attribute k : ScalarValues::Integer;
        attribute m : ScalarValues::Integer;
        action a : Step;
        bind k = m;
        then action b : Step;
    }
    part def Widget;
    port def Plug;
    action def AfterConnect {
        action a : Step;
        part p : Widget;
        part q : Widget;
        connect p to q;
        then action b : Step;
    }
    action def AfterConnection {
        action a : Step;
        part p : Widget;
        part q : Widget;
        connection c connect p to q;
        then action b : Step;
    }
    action def AfterInterface {
        action a : Step;
        port p1 : Plug;
        port p2 : ~Plug;
        interface i connect p1 to p2;
        then action b : Step;
    }
    action def AfterAllocate {
        action a : Step;
        part p : Widget;
        part q : Widget;
        allocate p to q;
        then action b : Step;
    }
    action def AfterAllocation {
        action a : Step;
        part p : Widget;
        part q : Widget;
        allocation al allocate p to q;
        then action b : Step;
    }
    part def AfterConnectors {
        part a : Widget;
        part p : Widget;
        part q : Widget;
        connect p to q;
        allocate p to q;
        then part b : Widget;
    }
    action def AfterAttribute {
        action a : Step;
        attribute k : ScalarValues::Integer;
        then action b : Step;
    }
    action def AfterDoc {
        action a : Step;
        doc /* a is followed by b */
        then action b : Step;
    }
    action def AfterSuccession {
        action a : Step;
        action c : Step;
        succession first a then c;
        then action b : Step;
    }
    action def AfterFirstThen {
        action a : Step;
        first a then c;
        then action b : Step;
        action c : Step;
    }
    action def AfterFirstStart {
        first start;
        then action a : Step;
        then done;
    }
    action stride : Step;
    action def AfterPerform {
        perform stride;
        flow from stride.y to b.x;
        then action b : Step;
    }
    action def ThenAlone {
        action a : Step;
        flow from a.y to b.x;
        then b;
        action b : Step;
    }
    state def Lamp {
        attribute level : ScalarValues::Integer;
        state on;
        doc /* on precedes off */
        transition first on then off;
        then state off;
    }
    state def Switch {
        entry;
        attribute pressed : ScalarValues::Boolean;
        then idle;
        state idle;
    }
}
