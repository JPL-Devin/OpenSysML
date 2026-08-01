// Phase C Behavioral Bodies Demo
// Showcases new parsing capabilities for calculations, constraints, 
// requirements, actions, and state machines.

package PhaseC {
    
    // ==========================================
    // PHASE C1: Calculation Bodies
    // ==========================================
    
    calc totalPrice {
        return basePrice * quantity * (1.0 + taxRate);
    }
    
    calc def DistanceFormula {
        return sqrt(dx * dx + dy * dy);
    }
    
    calc boundedValue {
        return min(max(input, lowerBound), upperBound);
    }
    
    // ==========================================
    // PHASE C1: Constraint Bodies
    // ==========================================
    
    constraint validRange {
        assert x >= 0;
        assert x <= 100;
        assume initialized;
    }
    
    constraint def NonNegative {
        assert not value < 0;
    }
    
    constraint safeBraking {
        assume vehicle.speed > 0;
        assert vehicle.brakeForce >= minimumRequired;
        assert not vehicle.brakes.worn;
    }
    
    // ==========================================
    // PHASE C2: Requirement Bodies
    // ==========================================
    
    requirement SafetyReq {
        subject vehicle : Vehicle;
        assume vehicle.speed > 0;
        require vehicle.brakes.functional;
        require vehicle.airbags.deployed;
        actor driver : Driver;
    }
    
    requirement def EmergencyStop {
        subject system : BrakingSystem;
        actor operator : HumanOperator;
        assume operator.alertLevel > threshold;
        require system.response < maxDelay;
        require system.force >= emergencyForce;
    }
    
    requirement PasswordPolicy {
        subject password : String;
        require password.length >= 8;
        require password.hasUpperCase;
        require password.hasDigit;
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
        then start greenLight;
        then greenLight yellowLight;
        then yellowLight redLight;
        then redLight greenLight;
    }
    
    action def ParallelProcessing {
        first start;
        fork splitWork;
        action processTask1;
        action processTask2;
        action processTask3;
        join synchronize;
        action aggregateResults;
        then start splitWork;
        then splitWork processTask1;
        then splitWork processTask2;
        then splitWork processTask3;
        then synchronize aggregateResults;
    }
    
    // ==========================================
    // PHASE C4: State Bodies
    // ==========================================
    
    state Running {
        entry { action initialize; }
        do { action processData; }
        exit { action cleanup; }
        
        state Active;
        state Idle;
        state Paused;
        
        transition Active to Idle when timeout;
        transition Idle to Active when userInput;
        transition Active to Paused if lowBattery;
    }
    
    state def ConnectionStateMachine {
        entry { 
            action openSocket;
            action authenticate;
        }
        
        state Disconnected;
        state Connecting;
        state Connected;
        state Error;
        
        transition Disconnected to Connecting when connectRequest;
        transition Connecting to Connected if authSuccess do {
            action logConnection;
            action notifyListeners;
        };
        transition Connecting to Error if authFailure do {
            action logError;
        };
        transition Connected to Disconnected when closeRequest do {
            action flushBuffers;
        };
        
        exit {
            action closeSocket;
        }
    }
    
    state VehicleOperating {
        entry { action startEngine; }
        
        state Idling;
        state Accelerating;
        state Cruising;
        state Braking;
        
        transition Idling to Accelerating when gasPedalPressed;
        transition Accelerating to Cruising if targetSpeedReached;
        transition Cruising to Braking when brakePedalPressed;
        transition Braking to Idling if speedZero do {
            action engageHandbrake;
        };
        transition Accelerating to Braking if emergencyDetected do {
            action activateABS;
            action deployAirbags;
        };
        
        exit { action shutdownEngine; }
    }
    
    // ==========================================
    // SIMPLE COMBINED EXAMPLE
    // ==========================================
    
    requirement def AutopilotSafety {
        subject autopilot : AutopilotSystem;
        actor vehicle : AutonomousVehicle;
        
        assume autopilot.calibrated;
        require autopilot.sensorHealth >= 0.95;
        require autopilot.responseTime < 100;
    }
    
    calc def RiskScore {
        return (1.0 - sensorHealth) * trafficDensity;
    }
    
    state def AutopilotMode {
        entry { action validateSensors; }
        
        state Monitoring;
        state Engaged;
        state Disengaging;
        
        transition Monitoring to Engaged if allSystemsReady do {
            action notifyDriver;
            action recordEngagement;
        };
        transition Engaged to Disengaging when driverOverride;
        transition Engaged to Disengaging if sensorFailure do {
            action alertDriver;
            action logIncident;
        };
        transition Disengaging to Monitoring if safeHandoff;
        
        exit { action generateReport; }
    }
}
