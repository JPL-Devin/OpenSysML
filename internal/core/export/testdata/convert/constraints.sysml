package Constraints {
    attribute def Mass;
    constraint def MassLimit {
        in m : Mass;
        in limit : Mass;
        m < limit
    }
    calc def Margin {
        in m : Mass;
        return : Mass = 100 - m;
    }
    part def Rover {
        attribute mass : Mass = 80;
        constraint limit : MassLimit {
            in m = mass;
            in limit = 100;
        }
        assert constraint massOK : MassLimit;
        assert not constraint massBad : MassLimit;
        assert constraint {
            mass > 0
        }
        constraint bounds {
            assert mass > 0;
            assume mass != 0;
            assert not mass > 1000;
            assert constraint inner {
                mass < 1000
            }
        }
        requirement lightEnough {
            subject r : Rover;
            assume r.mass > 0;
            require constraint {
                r.mass < 100
            }
            require r.mass < 200;
            require MassLimit;
            assume MassLimit {
                in m = r.mass;
                in limit = 100;
            }
        }
    }
}
