// A constraint body that declares parameters is read by parseCalcBody, so its
// asserted conditions are members of that body too — including conditions that
// start with a keyword — and `binding … of …` records a reference.
part def Tank {
    attribute level : Real;
    attribute full : Boolean;

    constraint notOverfull {
        in x : Real;
        assert x >= 0;
        assume not x > 100;
    }

    constraint keywordInitial {
        in x : Real;
        assert true;
        assert not false;
        assume null != x;
        assert if x > 0 ? true else false;
    }

    binding levelIsFull of full = level;
}
