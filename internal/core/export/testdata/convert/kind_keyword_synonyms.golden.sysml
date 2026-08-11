package Synonyms {
    datatype Temperature;
    feature reading : Temperature;
    function def Average;
    occurrence base;
    snapshot atStart;
    timeslice during;
    message note;
    allocate budget;
    use case def Drive;
    constraint def Safe;
    state def Idle;
    part def Vehicle {
        exhibit idling : Idle;
        assert safe : Safe;
        inv alsoSafe : Safe;
    }
    view def Dashboard {
        render display;
    }
}
