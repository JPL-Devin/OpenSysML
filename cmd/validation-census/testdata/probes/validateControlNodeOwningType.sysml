// Census: validateControlNodeOwningType
// Expect: error: which is not an action
package CN05ControlNodeOutsideAction {
    constraint def C {
        fork f;
    }
    part def Q {
        constraint c {
            decide d;
        }
    }
}
