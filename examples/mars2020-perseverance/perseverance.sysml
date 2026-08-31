// The Mars 2020 Perseverance rover: structure, interfaces, behavior,
// requirements, analyses and views. See README.md for the walkthrough.

// Shared value types, units, and mission-wide metadata.
package MarsSurfaceEnvironment {
    private import ScalarValues::*;
    private import SI::*;
    private import ISQ::*;

    // Marks elements whose failure ends the mission.
    metadata def MissionCritical;

    // Marks elements delivered by an instrument principal investigator team.
    metadata def SciencePayload;

    attribute def MarsLocalSolarTime {
        attribute sol : Integer;
        attribute hour : Real;
    }

    enum def OperationalMode {
        nominal;
        safeMode;
        sleep;
    }
}

// What flows through the rover: data, power, samples, and commands.
package RoverFlows {
    private import ScalarValues::*;

    item def Telemetry {
        attribute sizeBits : Real;
    }

    item def Command {
        attribute opcode : Integer;
    }

    item def ElectricalPower;

    item def RegolithSample {
        attribute sealed : Boolean;
    }

    item def FaultAlarm;

    item def MartianAtmosphere;

    item def Oxygen;
}

// The ports and interface definitions the subsystems connect through.
package RoverInterfaces {
    private import ScalarValues::*;
    private import RoverFlows::*;

    port def PowerPort {
        out item power : ElectricalPower;
        attribute voltage : Real;
    }

    port def DataPort {
        out item telemetry : Telemetry;
        in item command : Command;
    }

    port def RadioPort {
        out item downlink : Telemetry;
        in item uplink : Command;
    }

    port def SampleTransferPort {
        out item sample : RegolithSample;
    }

    interface def PowerBus {
        end source : PowerPort;
        end load : ~PowerPort;
    }

    interface def DataBus {
        end host : DataPort;
        end peripheral : ~DataPort;
    }

    interface def SpaceLink {
        end ground : RadioPort;
        end flight : ~RadioPort;
    }
}

// The rover's physical decomposition: every major subsystem as a part
// definition, assembled and connected in the Rover part def at the end.
package RoverStructure {
    private import ScalarValues::*;
    private import SI::*;
    private import ISQ::*;
    private import RealFunctions::sum;
    private import MarsSurfaceEnvironment::*;
    private import RoverFlows::*;
    private import RoverInterfaces::*;

    // --- Power ---

    // Multi-Mission Radioisotope Thermoelectric Generator.
    #MissionCritical part def MMRTG {
        attribute mass : MassValue = 45.0 [kg];
        attribute beginningOfLifePower : PowerValue = 110.0 [W];
        attribute plutoniumMass : MassValue = 4.8 [kg];
        port supply : PowerPort {
            attribute :>> voltage = 28.0;
        }
    }

    part def Battery {
        attribute mass : MassValue = 26.0 [kg];
        attribute capacityAmpHours : Real = 43.0;
        attribute stateOfCharge : Real = 1.0;
        port supply : PowerPort;
    }

    // --- Avionics ---

    // Rover Compute Element: a RAD750 flight computer.
    #MissionCritical part def RoverComputeElement {
        attribute clockMHz : Real = 200.0;
        attribute ramMB : Real = 256.0;
        port bus : DataPort;
        port power : ~PowerPort;
    }

    // --- Telecom: peripherals of the data bus, so their ends are conjugated ---

    part def UHFAntenna {
        attribute returnRateBitsPerSec : Real = 2000000.0;
        port link : ~RadioPort;
    }

    part def HighGainAntenna {
        attribute bandGHz : Real = 8.4;
        attribute returnRateBitsPerSec : Real = 800.0;
        port link : ~RadioPort;
    }

    part def LowGainAntenna {
        port link : ~RadioPort;
    }

    part def TelecomSubsystem {
        part uhf : UHFAntenna;
        part hga : HighGainAntenna;
        part lga : LowGainAntenna;
        port bus : ~DataPort;
        port power : ~PowerPort;
    }

    // --- Mobility ---

    part def Wheel {
        attribute diameter : LengthValue = 0.525 [m];
        attribute titaniumSpokes : Boolean = true;
    }

    part def RockerBogieSuspension {
        attribute enablesClimbOverObstacle : LengthValue = 0.4 [m];
    }

    part def MobilitySubsystem {
        attribute topSpeed : SpeedValue = 0.042 ['m/s'];
        part wheels : Wheel[6];
        part suspension : RockerBogieSuspension;
        port power : ~PowerPort;
        port bus : ~DataPort;
    }

    // --- Robotic arm and sampling ---

    part def Drill {
        attribute rotationalRateRPM : Real = 3000.0;
    }

    part def RoboticArm {
        attribute reach : LengthValue = 2.1 [m];
        attribute jointCount : Integer = 5;
        part drill : Drill;
        port power : ~PowerPort;
        port bus : ~DataPort;
    }

    part def SampleTube {
        attribute mass : MassValue = 0.057 [kg];
        attribute sealed : Boolean = false;
    }

    #MissionCritical part def SampleCachingSystem {
        part tubes : SampleTube[43];
        port sampleOut : SampleTransferPort;
        port power : ~PowerPort;
        port bus : ~DataPort;
    }

    // --- Science instruments ---

    #SciencePayload part def Mastcam_Z {
        attribute mass : MassValue = 4.0 [kg];
        attribute zoomFocalLengthMax : LengthValue = 0.11 [m];
        port bus : ~DataPort;
    }

    #SciencePayload part def SuperCam {
        attribute mass : MassValue = 10.6 [kg];
        attribute laserRange : LengthValue = 7.0 [m];
        port bus : ~DataPort;
    }

    #SciencePayload part def PIXL {
        attribute mass : MassValue = 6.9 [kg];
        port bus : ~DataPort;
    }

    #SciencePayload part def SHERLOC {
        attribute mass : MassValue = 4.7 [kg];
        port bus : ~DataPort;
    }

    #SciencePayload part def MEDA {
        attribute mass : MassValue = 5.5 [kg];
        port bus : ~DataPort;
    }

    #SciencePayload part def RIMFAX {
        attribute mass : MassValue = 3.0 [kg];
        attribute penetrationDepth : LengthValue = 10.0 [m];
        port bus : ~DataPort;
    }

    // In-Situ Resource Utilization Experiment: makes oxygen from CO2.
    #SciencePayload part def MOXIE {
        attribute mass : MassValue = 17.1 [kg];
        attribute oxygenRateGramsPerHour : Real = 10.0;
        in item intake : MartianAtmosphere;
        out item product : Oxygen;
        port power : ~PowerPort;
        port bus : ~DataPort;
    }

    part def ScienceInstrumentSuite {
        part mastcamZ : Mastcam_Z;
        part superCam : SuperCam;
        part pixl : PIXL;
        part sherloc : SHERLOC;
        part meda : MEDA;
        part rimfax : RIMFAX;
        part moxie : MOXIE;

        attribute payloadMass : MassValue =
            mastcamZ.mass + superCam.mass + pixl.mass + sherloc.mass
            + meda.mass + rimfax.mass + moxie.mass;
    }

    // --- The technology demonstration the rover carried ---

    part def IngenuityHelicopter {
        attribute mass : MassValue = 1.8 [kg];
        attribute rotorDiameter : LengthValue = 1.2 [m];
    }

    // --- The rover itself ---

    #MissionCritical part def Rover {
        attribute totalMass : MassValue = 1025.0 [kg];
        attribute length : LengthValue = 3.0 [m];
        attribute width : LengthValue = 2.7 [m];
        attribute height : LengthValue = 2.2 [m];
        attribute mode : OperationalMode := OperationalMode::nominal;

        part mmrtg : MMRTG;
        part batteries : Battery[2];
        part rce : RoverComputeElement[2];
        part telecom : TelecomSubsystem;
        part mobility : MobilitySubsystem;
        part arm : RoboticArm;
        part caching : SampleCachingSystem;
        part instruments : ScienceInstrumentSuite;
        part helicopter : IngenuityHelicopter;

        // Power distribution from the MMRTG to the loads.
        interface rceFeed : PowerBus connect mmrtg.supply to rce.power;
        interface mobilityFeed : PowerBus connect mmrtg.supply to mobility.power;
        interface telecomFeed : PowerBus connect mmrtg.supply to telecom.power;
        interface armFeed : PowerBus connect mmrtg.supply to arm.power;
        interface cachingFeed : PowerBus connect mmrtg.supply to caching.power;
        interface moxieFeed : PowerBus connect mmrtg.supply to instruments.moxie.power;

        // Data bus from the compute element hosts to the conjugated peripherals.
        connect rce.bus to telecom.bus;
        connect rce.bus to mobility.bus;
        connect rce.bus to arm.bus;
        connect rce.bus to caching.bus;

        flow of ElectricalPower from mmrtg.supply.power to rce.power.power;
        flow of Telemetry from rce.bus.telemetry to telecom.bus.telemetry;
    }
}

// The rover's behavior: its top-level operating states and the sol activities
// it performs, ending with the sample-caching action it exists to carry out.
package RoverBehavior {
    private import ScalarValues::*;
    private import SI::*;
    private import MarsSurfaceEnvironment::*;
    private import RoverFlows::*;
    private import RoverStructure::*;

    // Entry/descent/landing followed by surface operations.
    state def MissionPhases {
        entry; then cruise;

        state cruise;

        state edl {
            entry; then guidedEntry;

            state guidedEntry;
            state parachuteDescent;
            state poweredDescent;
            state skyCrane;

            transition first guidedEntry then parachuteDescent;
            transition first parachuteDescent then poweredDescent;
            transition first poweredDescent then skyCrane;
        }

        state surfaceOps;

        transition first cruise then edl;
        transition first edl then surfaceOps;
    }

    // The rover's day-to-day operating states while on the surface.
    state def SurfaceStates {
        attribute batteryCharge : Real = 1.0;

        entry; then awake;

        state awake {
            entry; then idle;

            state idle;
            state driving;
            state sampling;
            state communicating;

            transition first idle accept Command then driving;
            transition first driving then idle;
            transition first idle accept after 10 [s] then sampling;
            transition first sampling accept after 20 [s] then communicating;
            transition first communicating accept after 5 [s] then idle;
        }

        state asleep;
        state safe;

        transition awake_to_asleep
            first awake
            if batteryCharge < 0.2
            then asleep;

        transition first asleep accept after 30 [s] then awake;
        transition first awake accept FaultAlarm then safe;
    }

    // One sol of surface operations, planned on Earth and run on Mars.
    action def ExecuteSolPlan {
        in item uplinkedPlan : Command;

        first start;

        action wakeUp;

        // Science and driving proceed in parallel with environment monitoring.
        fork solActivities;

        action driveToTarget {
            action planPath;
            action executeDrive;
            action assessTerrain;
            first planPath;
            succession first planPath then executeDrive;
            succession first executeDrive then assessTerrain;
        }

        action monitorWeather;

        join solComplete;

        action downlinkTelemetry {
            out item telemetry : Telemetry;
        }

        action sleepEarly;
        action nightScience;

        succession first start then wakeUp;
        succession first wakeUp then solActivities;
        succession first solActivities then driveToTarget;
        succession first solActivities then monitorWeather;
        succession first driveToTarget then solComplete;
        succession first monitorWeather then solComplete;
        succession first solComplete then downlinkTelemetry;
        succession first downlinkTelemetry then checkBattery;
        decide checkBattery;
        if true then nightScience;
        else sleepEarly;
        succession first nightScience then done;
        succession first sleepEarly then done;
    }

    // The sampling sequence: abrade, drill, seal, cache.
    action def AcquireSample {
        in target : String;
        out item cachedSample : RegolithSample;

        first start;

        action abradeSurface;
        action drillCore;
        action transferToCarousel;
        action sealTube {
            assign cachedSample.sealed := true;
        }
        action storeTube;

        succession first start then abradeSurface;
        succession first abradeSurface then drillCore;
        succession first drillCore then transferToCarousel;
        succession first transferToCarousel then sealTube;
        succession first sealTube then storeTube;
        succession first storeTube then done;
    }

    // MOXIE's oxygen-production run as a calculation.
    calc def OxygenProduced {
        in ratePerHour : Real;
        in hours : Real;
        return attribute grams : Real = ratePerHour * hours;
    }
}

// The mission's requirements, the use case they serve, and their verification.
package RoverRequirements {
    private import ScalarValues::*;
    private import SI::*;
    private import ISQ::*;
    private import VerificationCases::VerdictKind;
    private import SequenceFunctions::size;
    private import MarsSurfaceEnvironment::*;
    private import RoverStructure::*;

    requirement def LandedMassLimit {
        doc /* The rover's landed mass shall not exceed what the EDL system
             * can deliver to the surface. */
        subject rover : Rover;
        attribute maxLandedMass : MassValue = 1050.0 [kg];

        require constraint {
            rover.totalMass <= maxLandedMass
        }
    }

    requirement def PowerMargin {
        doc /* The MMRTG shall supply the rover's continuous load with margin. */
        subject rover : Rover;
        attribute continuousLoad : PowerValue = 90.0 [W];

        require constraint {
            rover.mmrtg.beginningOfLifePower >= continuousLoad
        }
    }

    requirement def SampleCapacity {
        doc /* The caching system shall carry enough tubes for the notional
             * 38-sample return cache plus witness tubes. */
        subject rover : Rover;

        require constraint {
            size(rover.caching.tubes) >= 43
        }
    }

    requirement def InstrumentAccommodation {
        doc /* The science payload mass shall stay within its allocation. */
        subject rover : Rover;
        attribute payloadAllocation : MassValue = 59.0 [kg];

        require constraint {
            rover.instruments.payloadMass <= payloadAllocation
        }
    }

    // The mission itself, with the actors that run it.
    use case def SeekSignsOfAncientLife {
        subject rover : Rover;
        actor missionOperations;
        actor scienceTeam;

        objective {
            doc /* Collect and cache scientifically selected samples of Jezero
                 * crater for return to Earth, and characterize the site's
                 * geology and past habitability. */
        }
    }

    part perseverance : Rover;

    use case mars2020 : SeekSignsOfAncientLife {
        subject rover = perseverance;
    }

    requirement massHolds : LandedMassLimit {
        subject rover = perseverance;
    }

    requirement powerHolds : PowerMargin {
        subject rover = perseverance;
    }

    requirement tubesHold : SampleCapacity {
        subject rover = perseverance;
    }

    requirement payloadHolds : InstrumentAccommodation {
        subject rover = perseverance;
    }

    assert satisfy massHolds by perseverance;

    verification def VerifyLandedMass {
        subject rover : Rover;

        objective {
            verify massHolds;
        }

        return attribute verdict : VerdictKind;
    }

    verification verifyLandedMass : VerifyLandedMass {
        subject rover = perseverance;
    }
}

// Trade studies the solver can run over the model.
package RoverAnalyses {
    private import ScalarValues::*;
    private import SI::*;
    private import ISQ::*;
    private import TradeStudies::*;
    private import RoverStructure::*;

    // How much science payload fits once the bus is accounted for.
    analysis def PayloadMassTrade {
        attribute payloadMass : MassValue;
        attribute busMass : MassValue;

        objective heaviestPayload : MaximizeObjective {
            subject :>> selectedAlternative;

            require constraint {
                payloadMass + busMass <= 1050.0 [kg]
            }

            require constraint {
                busMass >= 966.0 [kg]
            }

            attribute :>> best = payloadMass;
        }
    }
}

// The individual rover that landed, and the milestones of its mission.
package Mars2020Mission {
    private import ScalarValues::*;
    private import MarsSurfaceEnvironment::*;
    private import RoverStructure::*;

    occurrence def SurfaceMission {
        attribute solsCompleted : Integer;
        attribute samplesCached : Integer;
        ref part vehicle : Rover;
    }

    individual def PerseveranceMission :> SurfaceMission;

    individual mission : PerseveranceMission {
        // Touchdown in Jezero crater, 2021-02-18.
        snapshot landing {
            attribute :>> solsCompleted = 0;
            attribute :>> samplesCached = 0;
        }

        // The prime mission: one Mars year of surface operations.
        timeslice primeMission {
            attribute :>> solsCompleted = 669;
        }
    }
}

// Views over the model, one per stakeholder question.
package RoverViews {
    private import Views::*;
    private import StandardViewDefinitions::*;

    view structure {
        expose RoverStructure::Rover;
        render asInterconnectionDiagram;
    }

    view partsTable {
        expose RoverStructure::Rover;
        render asElementTable;
    }

    view surfaceStates : StateTransitionView {
        expose RoverBehavior::SurfaceStates;
    }

    view solPlan : ActionFlowView {
        expose RoverBehavior::ExecuteSolPlan;
    }

    // Only the elements whose loss ends the mission.
    view criticalItems {
        expose RoverStructure::**[@MarsSurfaceEnvironment::MissionCritical];
    }

    // Only the PI-delivered instruments.
    view payloadView {
        expose RoverStructure::**[@MarsSurfaceEnvironment::SciencePayload];
    }
}
