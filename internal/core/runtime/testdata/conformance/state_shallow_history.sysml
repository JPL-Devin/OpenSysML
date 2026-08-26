// A shallow history re-enters the substate of its composite state that was
// active when that state was last exited, rather than the default one its
// history transition names.
package Test {
    state ShallowHistory {
        initial start;
        state outer {
            state first;
            state second;
            history resume;
        }
        state away;
        final done;

        succession first start then first;

        transition first to second when Next;
        transition second to away when Leave;
        transition away to resume when Back;
        transition resume to first;
        transition second to done when Finish;
    }
}
