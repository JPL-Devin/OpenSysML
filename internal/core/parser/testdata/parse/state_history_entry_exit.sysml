package HistoryAndPoints {
    state Controller {
        entry; then start;
        state start;
        state outer {
            state first;
            state second;
            history resume;
            deep history resumeDeep;
            entry point into;
            exit point outOf;
        }
        state away;

        succession first start then first;

        transition first first when Next then second;
        transition first second when Leave then away;
        transition first away when Back then resume;
    }
}
