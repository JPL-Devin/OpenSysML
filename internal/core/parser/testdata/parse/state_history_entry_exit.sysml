package HistoryAndPoints {
    state Controller {
        initial start;
        state outer {
            state first;
            state second;
            history resume;
            deep history resumeDeep;
            entry point into;
            exit point outOf;
        }
        state away;

        start then first;

        transition first to second when Next;
        transition second to away when Leave;
        transition away to resume when Back;
    }
}
