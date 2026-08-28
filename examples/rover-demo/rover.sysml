// A rover in four packages: what it is made of, what it does, the budget a
// solver can answer about, and the views it is presented through. See README.md
// for the walkthrough that drives it from the CLI, the REPL and Python.

// The rover itself: its parts, how power and data move through them, and which
// of them are mission-critical.
package Rover {
    private import ScalarValues::*;

    // Marks the parts a safety review has to look at, so a view can expose just
    // those with an element filter.
    metadata def Critical;

    enum def DriveMode {
        enum parked;
        enum creep;
        enum nominal;
    }

    enum def Band {
        enum uhf;
        enum xBand;
    }

    item def Charge;

    item def Frame;

    item def Sample;

    port def PowerPort {
        in item drawn : Charge;
    }

    port def DataPort {
        in item frames : Frame;
    }

    connection def PowerLine {
        end source : PowerPort;
        end sink : PowerPort;
    }

    connection def DataLink {
        end producer : DataPort;
        end consumer : DataPort;
    }

    #Critical part def Battery {
        attribute capacity : Real = 1600.0;
        attribute charge : Real = 1200.0;

        out item supplied : Charge;
        port outlet : PowerPort;
    }

    part def SolarArray {
        attribute area : Real = 2.6;
        attribute output : Real = 340.0;

        out item generated : Charge;
        port outlet : PowerPort;
    }

    #Critical part def Wheel {
        attribute diameter : Real = 0.52;
        attribute steerable : Boolean = true;
    }

    #Critical part def Mobility {
        attribute drivePower : Real = 120.0;
        attribute speed : Real = 0.042;

        part wheels : Wheel[6];

        in item drawn : Charge;
        port feed : PowerPort;
    }

    part def Camera {
        attribute megapixels : Real = 20.2;

        out item captured : Frame;
        port imagery : DataPort;
    }

    part def Mast {
        attribute height : Real = 2.2;

        part camera : Camera;
    }

    part def Drill {
        attribute depth : Real = 0.07;

        out item cores : Sample;
    }

    part def Arm {
        attribute reach : Real = 2.1;

        part drill : Drill;
    }

    #Critical part def Computer {
        attribute sciencePower : Real = 60.0;

        attribute framesHeld : Integer = 0;

        in item frames : Frame;
        port imagery : DataPort;
    }

    part def Antenna {
        attribute band : Band = Band::xBand;
        attribute gain : Real = 26.0;
    }

    // The rover as a whole: the parts above, the power line and data link between
    // them, and the flows of charge and imagery those connections carry.
    part def Platform {
        attribute mass : Real default 899.0;
        attribute mode : DriveMode = DriveMode::parked;

        part battery : Battery;
        part array : SolarArray;
        part mobility : Mobility;
        part mast : Mast;
        part arm : Arm;
        part computer : Computer;
        part antenna : Antenna;

        connection drivePower : PowerLine connect battery.outlet to mobility.feed;
        connection imageryLink : DataLink connect mast.camera.imagery to computer.imagery;

        flow of Charge from battery.supplied to mobility.drawn;
        flow of Frame from mast.camera.captured to computer.frames;

        // What the launch vehicle allows, checked per rover rather than per class.
        assert constraint withinMassBudget {
            mass <= 1000.0
        }
    }

    // Two rovers, so a check has something to disagree about: the flight article
    // is within its budgets and the engineering model is over its mass budget.
    part curiosity : Platform {
        attribute :>> mass = 899.0;
    }

    part heavyMockup : Platform {
        attribute :>> mass = 1120.0;
    }
}

// What the rover does: the calculations a traverse is planned with, the action
// that runs a sol of surface operations, and the machine the rover's modes are.
package RoverBehavior {
    private import ScalarValues::*;
    private import RealFunctions::sqrt;
    private import RealFunctions::min;
    private import SI::s;

    // Straight-line distance between two waypoints, in metres.
    calc def Traverse {
        in dx : Real;
        in dy : Real;

        sqrt(dx * dx + dy * dy)
    }

    // Watt-hours a drive of that many metres costs at the given draw.
    calc energyForDrive {
        in metres : Real;
        in speed : Real;
        in drivePower : Real;

        drivePower * (metres / speed) / 3600.0
    }

    // How much of the day's science the downlink can carry home.
    calc downlinkable {
        in framesHeld : Integer;
        in passMinutes : Real;

        min(framesHeld, passMinutes * 4.0)
    }

    // A sol of surface operations. The fork runs imaging and sampling as steps of
    // the one performance, the join waits for both, and the decision at the end
    // downlinks only what the day's power budget left room for.
    action def SurfaceOps {
        in attribute chargeAvailable : Real = 1200.0;

        attribute imageFrames : Integer = 0;
        attribute coreFrames : Integer = 0;
        attribute imagingCost : Real = 0.0;
        attribute samplingCost : Real = 0.0;

        out attribute framesHeld : Integer;
        out attribute downlinked : Integer;
        out attribute chargeLeft : Real;

        first start;

        action wake {
            assign downlinked := 0;
        }

        fork split;

        // The two branches write features of their own, so what the join sees does
        // not depend on the order the branches were interleaved in.
        action image {
            assign imageFrames := 12;
            assign imagingCost := 40.0;
        }

        // The other branch is two steps of its own: the drill runs, then the core
        // is stowed.
        action drill {
            assign samplingCost := 120.0;
        }

        action stow {
            assign coreFrames := 3;
        }

        join sync;

        action tally {
            assign framesHeld := imageFrames + coreFrames;
            assign chargeLeft := chargeAvailable - imagingCost - samplingCost;
        }

        action downlink {
            assign downlinked := framesHeld;
        }

        action deferDownlink {
            assign downlinked := 0;
        }

        succession first start then wake;
        succession first wake then split;
        succession first split then image;
        succession first split then drill;
        succession first drill then stow;
        succession first image then sync;
        succession first stow then sync;
        succession first sync then tally;

        // Downlink only what the day's power budget left room for.
        succession first tally then budget;
        decide budget;
        if chargeLeft > 900.0 then downlink;
        else deferDownlink;

        succession first downlink then done;
        succession first deferDownlink then done;
    }

    // The rover's modes. The timed triggers are what `%advance` dispatches, and
    // each entry action writes a feature of the object exhibiting the machine.
    state def Modes {
        attribute odometer : Real = 0.0;
        attribute framesCaptured : Integer = 0;
        attribute faults : Integer = 0;

        entry; then idle;

        state idle;

        // Driving is a state with substates of its own: entering `rolling` enters
        // `driving` around it, and the guarded transition out of `rolling` is what
        // a fault takes.
        state driving {
            state rolling {
                entry action roll {
                    assign odometer := odometer + 21.0;
                }
            }

            state holding;
        }

        state science {
            entry action observe {
                assign framesCaptured := framesCaptured + 12;
            }
        }

        state safing {
            entry action record {
                assign faults := faults + 1;
            }
        }

        // Entering the substate is what enters `driving`, so a transition into it
        // names the substate through the state around it.
        transition idle_to_rolling
            first idle
            accept after 5 [s]
            then driving.rolling;

        transition rolling_to_holding
            first driving.rolling
            accept after 10 [s]
            then driving.holding;

        transition rolling_to_safe
            first driving.rolling
            if faults > 0
            then safing;

        transition driving_to_science
            first driving
            accept after 20 [s]
            then science;

        transition first science then done;
    }

    // An object of this type runs the machine and performs the sol: `%instantiate`
    // starts both, and `%invoke` calls the operation below on that object.
    part def RoverController {
        attribute odometer : Real = 0.0;
        attribute framesHeld : Integer = 0;

        exhibit state modes : Modes;

        action recordPass {
            in attribute frames : Integer;

            action store {
                assign framesHeld := framesHeld + frames;
            }

            first store;
        }
    }

    part controller : RoverController;
}

// The rover's sol budget written as conditions a solver can answer about: how
// the day's energy is split, which plans cannot hold, which rover the mission
// can be configured as, and the best plan the budget admits.
package RoverSolver {
    private import ScalarValues::*;
    public import SI::*;
    public import ISQ::*;
    private import TradeStudies::*;

    doc /* Energy is in watt-hours, mass in kilograms, distance in metres. */

    // A plan for one sol. `%check` answers sat and `%solve` fills in the three
    // draws, keeping the energy the battery already states.
    part def SolPlan {
        attribute energyAvailable : Integer = 1200;
        attribute driveEnergy : Integer;
        attribute scienceEnergy : Integer;
        attribute housekeepingEnergy : Integer;

        assert constraint fitsTheBattery {
            driveEnergy + scienceEnergy + housekeepingEnergy <= energyAvailable
        }

        assert constraint keepsTheRoverAlive {
            housekeepingEnergy >= 300
        }

        assert constraint driveIsWorthIt {
            driveEnergy >= 200
        }

        assert constraint scienceIsWorthIt {
            scienceEnergy >= 150
        }
    }

    // One requirement states the whole budget, so a single query is about all of
    // it: `%check` answers sat and `%solve` gives a witness plan.
    requirement def SolBudget {
        subject plan : SolPlan;

        require constraint {
            plan.driveEnergy + plan.scienceEnergy + plan.housekeepingEnergy <= 1200
        }

        require constraint {
            plan.housekeepingEnergy >= 300
        }

        require constraint {
            plan.driveEnergy >= 200
        }

        require constraint {
            plan.scienceEnergy >= 150
        }
    }

    // A sol nobody can plan: the three draws asked for exceed the battery.
    // `%check` answers unsat and `%explain` names the conditions that conflict.
    requirement def OverbookedSol {
        subject plan : SolPlan;

        require constraint {
            plan.driveEnergy + plan.scienceEnergy + plan.housekeepingEnergy <= 1200
        }

        require constraint {
            plan.driveEnergy >= 600
        }

        require constraint {
            plan.scienceEnergy >= 500
        }

        require constraint {
            plan.housekeepingEnergy >= 300
        }
    }

    requirement solIsBudgeted : SolBudget;

    // Two plans the budget is asserted of: `%satisfy` reports the one that holds
    // and the condition the other one fails.
    part sol42 : SolPlan {
        attribute :>> driveEnergy = 420;
        attribute :>> scienceEnergy = 380;
        attribute :>> housekeepingEnergy = 340;
    }

    part sol43 : SolPlan {
        attribute :>> driveEnergy = 700;
        attribute :>> scienceEnergy = 380;
        attribute :>> housekeepingEnergy = 340;
    }

    part plans {
        assert satisfy solIsBudgeted by sol42;
        assert satisfy solIsBudgeted by sol43;
    }

    attribute def Wheels;

    attribute def Antenna;

    attribute def Instrument;

    part def RoverBuild {
        attribute wheels : Wheels;
        attribute antenna : Antenna;
        attribute instrument : Instrument;
    }

    // Three variation points whose choices interact, so `%configure` synthesises
    // a consistent rover and `%configure … all` enumerates the ones that are.
    part roverFamily : RoverBuild {
        variation attribute :>> wheels {
            variant attribute aluminium;
            variant attribute titanium;
        }

        variation attribute :>> antenna {
            variant attribute uhfOnly;
            variant attribute xBand;
        }

        variation attribute :>> instrument {
            variant attribute camera;
            variant attribute drill;
        }

        // A drill's cores are only worth taking if they can be sent home, and the
        // heavier wheels are what carries a drill over rock.
        assert constraint buildIsConsistent {
            (instrument == instrument::drill implies antenna == antenna::xBand)
                and (instrument == instrument::drill implies wheels == wheels::titanium)
        }
    }

    // The most science a sol can hold: `%optimize` improves the objective within
    // the conditions it states.
    analysis def BestSol {
        attribute driveEnergy : Integer;
        attribute scienceEnergy : Integer;
        attribute housekeepingEnergy : Integer;

        objective mostScience : MaximizeObjective {
            require constraint {
                driveEnergy + scienceEnergy + housekeepingEnergy <= 1200
            }

            require constraint {
                housekeepingEnergy >= 300
            }

            // The rover has to reach the outcrop before it can study it.
            require constraint {
                driveEnergy >= 200
            }

            attribute :>> best = scienceEnergy;
        }
    }

    // A quantity-valued objective: the optimum is reported in the unit the
    // conditions are written in.
    analysis def LightestRover {
        attribute mass : ISQ::MassValue;

        objective lightest : MinimizeObjective {
            require constraint {
                mass >= 830 [kg]
            }

            require constraint {
                mass <= 1100 [kg]
            }

            attribute :>> best = mass;
        }
    }

    // Two objectives, improved lexicographically in the order declared: the
    // longest traverse first, and among the plans reaching it the most science.
    analysis def FarthestThenScience {
        attribute traverse : Integer;
        attribute scienceEnergy : Integer;

        objective farthest : MaximizeObjective {
            require constraint {
                traverse >= 20
            }

            require constraint {
                traverse <= 120
            }

            // Every metre driven costs energy science then cannot have.
            require constraint {
                scienceEnergy <= 900 - 4 * traverse
            }

            require constraint {
                scienceEnergy >= 150
            }

            attribute :>> best = traverse;
        }

        objective mostScience : MaximizeObjective {
            attribute :>> best = scienceEnergy;
        }
    }
}

// How the rover model is presented: one view per rendering kind, a viewpoint the
// exposed rovers are checked against, and a view that reaches only the parts
// marked critical.
package RoverViews {
    private import Views::*;
    private import StandardViewDefinitions::*;

    // What a reviewer wants of any rover exposed: it has to fly.
    concern def MassBudget {
        subject rover : Rover::Platform;
        attribute maxMass = 1000.0;
        require constraint {
            rover.mass <= maxMass
        }
    }

    viewpoint def OperationsPerspective {
        frame concern mass : MassBudget;
    }

    viewpoint operationsPerspective : OperationsPerspective;

    // The parts of the platform and the connections between them.
    view interfaces {
        expose Rover::Platform;
        render asInterconnectionDiagram;
    }

    // States no rendering, so a containment tree renders it, with the
    // interconnection view nested as a subtree. `%view` checks the two rovers
    // exposed here against the concern the viewpoint frames: one is within the
    // mass budget and the mock-up is over it.
    view overview {
        expose Rover::curiosity;
        expose Rover::heavyMockup;
        view interfaceSubview :> interfaces;

        satisfy operationsPerspective;
        frame concern mass : MassBudget;
    }

    // The rendering kind of each of these comes from the standard view
    // definition it specializes.
    view modes : StateTransitionView {
        expose RoverBehavior::Modes;
    }

    view sol : ActionFlowView {
        expose RoverBehavior::SurfaceOps;
    }

    view partsTable {
        expose Rover::Platform;
        render asElementTable;
    }

    // A filtered recursive expose, which reaches only the parts a fault would
    // end the mission at.
    view criticalParts {
        expose Rover::**[@Rover::Critical];
    }
}
