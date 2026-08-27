// The orthogonal regions of a composite state complete like the machine's own:
// the machine stays active while one of `busy`'s regions still runs, and
// completes once both reach `done`.
package test {
    attribute def First;
    attribute def Second;

    state sm {
        attribute log : String = "";

        entry; then busy;
        state busy parallel {
            state left {
                entry; then lstart;
                state lstart {
                    exit action { assign log := log + "l;"; }
                }
                transition first lstart accept First then done;
            }
            state right {
                entry; then rstart;
                state rstart {
                    exit action { assign log := log + "r;"; }
                }
                transition first rstart accept Second then done;
            }
        }
    }
}
