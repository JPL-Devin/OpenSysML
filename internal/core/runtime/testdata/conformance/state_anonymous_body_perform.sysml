package test {
    // A statement of an anonymous inline entry/do/exit body may perform an action
    // (SysML 7.17.6), so a body mixes its own statements with performed actions:
    // counter ends at 121 only if every statement of every body ran in order.
    action def Bump {
        inout counter : Integer;

        first start;

        action bumping {
            assign counter := counter * 10;
        }

        done end;

        then start bumping;
        then bumping end;
    }

    state TestAnonymousBodyPerform {
        attribute counter : Integer = 0;

        initial init;

        state active {
            entry action {
                assign counter := counter + 1;
                perform Bump;
            }
            exit action {
                perform Bump;
                assign counter := counter + 1;
            }
        }

        final done;

        init then active;
        active then done;
    }
}
