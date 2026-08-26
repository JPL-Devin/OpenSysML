package test {
    // A transition's effect may be a performed action (SysML 7.19.4
    // TransitionUsage effect behavior): `do perform Bump;` invokes Bump as the
    // transition fires, so `counter` ends at 11 only if the effect ran.
    action def Bump {
        inout counter : Integer;

        first start;

        action bumping {
            assign counter := counter + 10;
        }

        done;

        succession first start then bumping;
        succession first bumping then done;
    }

    state TestTransitionEffectPerform {
        attribute counter : Integer = 1;

        initial init;
        state active;
        final done;

        succession first init then active;
        transition active to done do perform Bump;
    }
}
