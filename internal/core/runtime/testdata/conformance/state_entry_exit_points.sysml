// Entry and exit points route a transition into and out of a composite state
// like a junction does: the machine enters `outer` through `into` and leaves it
// through `outOf`.
package Test {
    state EntryExitPoints {
        entry; then start;
        state start;
        state outer {
            state inner;
        }
        entry point into;
        exit point outOf;
        final done;

        transition first start then into;
        transition first into then inner;
        transition first inner when Leave then outOf;
        transition first outOf then done;
    }
}
