// Entry and exit points route a transition into and out of a composite state
// like a junction does: the machine enters `outer` through `into` and leaves it
// through `outOf`.
package Test {
    state EntryExitPoints {
        initial start;
        state outer {
            state inner;
        }
        entry point into;
        exit point outOf;
        final done;

        transition start to into;
        transition into to inner;
        transition inner to outOf when Leave;
        transition outOf to done;
    }
}
