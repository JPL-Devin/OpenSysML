state TimedTriggers {
    initial start;
    state waiting {
        accept after 10 then working;
    }
    state working {
        accept at 15 then done;
        accept when working then done;
    }
    final done;

    start then waiting;
}
