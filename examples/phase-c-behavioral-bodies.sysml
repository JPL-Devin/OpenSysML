// Phase C Behavioral Bodies Demo
// Showcases new parsing capabilities for calculations, constraints, 
// requirements, actions, and state machines.

package PhaseC {
    
    // The library is not implicitly visible: a bare name needs an import.
    private import ScalarValues::*;
    private import RealFunctions::sqrt;
    private import RealFunctions::min;
    private import RealFunctions::max;
    
    // The definitions the bodies below refer to.
    part def Vehicle {
        attribute speed : Real;
        attribute brakeForce : Real;
        part brakes {
            attribute worn : Boolean;
            attribute functional : Boolean;
        }
        part airbags {
            attribute deployed : Boolean;
        }
    }
    
    part def Driver;
    part def AuthenticatedUser;
    part def AutonomousVehicle;
    
    part def BrakingSystem {
        attribute response : Real;
        attribute force : Real;
    }
    
    part def HumanOperator {
        attribute alertLevel : Real;
    }
    
    part def AutopilotSystem {
        attribute calibrated : Boolean;
        attribute sensorHealth : Real;
        attribute responseTime : Real;
    }
    
    attribute def Password {
        attribute length : Integer;
        attribute hasUpperCase : Boolean;
        attribute hasDigit : Boolean;
    }
    
    // ==========================================
    // PHASE C1: Calculation Bodies
    // ==========================================
    
    calc totalPrice {
        in basePrice : Real;
        in quantity : Integer;
        in taxRate : Real;
        
        basePrice * quantity * (1.0 + taxRate)
    }
    
    calc def DistanceFormula {
        in dx : Real;
        in dy : Real;
        
        sqrt(dx * dx + dy * dy)
    }
    
    calc boundedValue {
        in input : Real;
        in lowerBound : Real;
        in upperBound : Real;
        
        min(max(input, lowerBound), upperBound)
    }
    
    // ==========================================
    // PHASE C1: Constraint Bodies
    // ==========================================
    
    // An assumption belongs to a requirement: a constraint body is one condition.
    requirement validRange {
        attribute x : Real;
        attribute initialized : Boolean;

        assume constraint { initialized }
        require constraint { x >= 0 and x <= 100 }
    }
    
    constraint def NonNegative {
        in value : Real;

        not (value < 0)
    }
    
    requirement safeBraking {
        subject vehicle : Vehicle;
        attribute minimumRequired : Real;

        assume constraint { vehicle.speed > 0 }
        require constraint { vehicle.brakeForce >= minimumRequired }
        require constraint { not vehicle.brakes.worn }
    }
    
    // ==========================================
    // PHASE C2: Requirement Bodies
    // ==========================================
    
    requirement SafetyReq {
        subject vehicle : Vehicle;
        assume constraint { vehicle.speed > 0 }
        require constraint { vehicle.brakes.functional }
        require constraint { vehicle.airbags.deployed }
        actor driver : Driver;
    }
    
    requirement def EmergencyStop {
        attribute threshold : Real;
        attribute maxDelay : Real;
        attribute emergencyForce : Real;
        
        subject system : BrakingSystem;
        actor operator : HumanOperator;
        assume constraint { operator.alertLevel > threshold }
        require constraint { system.response < maxDelay }
        require constraint { system.force >= emergencyForce }
    }
    
    requirement PasswordPolicy {
        subject password : Password;
        require constraint { password.length >= 8 }
        require constraint { password.hasUpperCase }
        require constraint { password.hasDigit }
        actor user : AuthenticatedUser;
    }
    
    // ==========================================
    // PHASE C3: Action Bodies (pre-existing)
    // ==========================================
    
    action TrafficLightSequence {
        first start;
        action greenLight;
        action yellowLight;
        action redLight;
        succession first start then greenLight;
        succession first greenLight then yellowLight;
        succession first yellowLight then redLight;
        succession first redLight then greenLight;
    }
    
    action def ParallelProcessing {
        first start;
        fork splitWork;
        action processTask1;
        action processTask2;
        action processTask3;
        join synchronize;
        action aggregateResults;
        succession first start then splitWork;
        succession first splitWork then processTask1;
        succession first splitWork then processTask2;
        succession first splitWork then processTask3;
        succession first synchronize then aggregateResults;
    }
    
    // ==========================================
    // PHASE C4: State Bodies
    // ==========================================
    
    state Running {
        attribute lowBattery : Boolean = false;
        attribute timeout : Boolean = false;
        attribute userInput : Boolean = false;
        
        entry action initialize;
        do action processData;
        exit action cleanup;
        
        state Active;
        state Idle;
        state Paused;
        
        // A transition out of the entry action names the state the machine starts in.
        transition initialize then Active;
        transition first Active accept when timeout then Idle;
        transition first Idle accept when userInput then Active;
        transition first Active if lowBattery then Paused;
    }
    
    state def ConnectionStateMachine {
        attribute authSuccess : Boolean = false;
        attribute authFailure : Boolean = false;
        attribute connectRequest : Boolean = false;
        attribute closeRequest : Boolean = false;
        
        entry action establishSession {
            action openSocket;
            action authenticate;
        }
        
        state Disconnected;
        state Connecting;
        state Connected;
        state Error;
        
        transition establishSession then Disconnected;
        transition first Disconnected accept when connectRequest then Connecting;
        transition first Connecting if authSuccess do action announce {
            action logConnection;
            action notifyListeners;
        } then Connected;
        transition first Connecting if authFailure do action reportFailure {
            action logError;
        } then Error;
        transition first Connected accept when closeRequest do action drain {
            action flushBuffers;
        } then Disconnected;
        
        exit action closeSocket;
    }
    
    state VehicleOperating {
        attribute targetSpeedReached : Boolean = false;
        attribute speedZero : Boolean = false;
        attribute emergencyDetected : Boolean = false;
        attribute gasPedalPressed : Boolean = false;
        attribute brakePedalPressed : Boolean = false;
        
        entry action startEngine;
        
        state Idling;
        state Accelerating;
        state Cruising;
        state Braking;
        
        transition startEngine then Idling;
        transition first Idling accept when gasPedalPressed then Accelerating;
        transition first Accelerating if targetSpeedReached then Cruising;
        transition first Cruising accept when brakePedalPressed then Braking;
        transition first Braking if speedZero do action handbrake {
            action engageHandbrake;
        } then Idling;
        transition first Accelerating if emergencyDetected do action emergencyResponse {
            action activateABS;
            action deployAirbags;
        } then Braking;
        
        exit action shutdownEngine;
    }
    
    // ==========================================
    // SIMPLE COMBINED EXAMPLE
    // ==========================================
    
    requirement def AutopilotSafety {
        subject autopilot : AutopilotSystem;
        actor vehicle : AutonomousVehicle;
        
        assume constraint { autopilot.calibrated }
        require constraint { autopilot.sensorHealth >= 0.95 }
        require constraint { autopilot.responseTime < 100 }
    }
    
    calc def RiskScore {
        in sensorHealth : Real;
        in trafficDensity : Real;
        
        (1.0 - sensorHealth) * trafficDensity
    }
    
    state def AutopilotMode {
        attribute allSystemsReady : Boolean = false;
        attribute sensorFailure : Boolean = false;
        attribute safeHandoff : Boolean = false;
        attribute driverOverride : Boolean = false;
        
        entry action validateSensors;
        
        state Monitoring;
        state Engaged;
        state Disengaging;
        
        transition validateSensors then Monitoring;
        transition first Monitoring if allSystemsReady do action engage {
            action notifyDriver;
            action recordEngagement;
        } then Engaged;
        transition first Engaged accept when driverOverride then Disengaging;
        transition first Engaged if sensorFailure do action reportFault {
            action alertDriver;
            action logIncident;
        } then Disengaging;
        transition first Disengaging if safeHandoff then Monitoring;
        
        exit action generateReport;
    }
}
