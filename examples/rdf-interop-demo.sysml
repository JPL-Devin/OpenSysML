// Structure only, so `sysml -convert ttl` round-trips it: see
// docs/RDF_INTEROP.md for what the mapping covers.
package RdfInteropDemo {
    doc /* A rover and its ground link, in structure only. */

    part def Battery {
        attribute capacity = 1500.0;
        attribute charge = 1500.0;
    }

    port def PowerPort {
        attribute volts = 28.0;
    }

    port def TelemetryPort;

    item def TelemetryFrame {
        attribute octets = 512;
    }

    part def Wheel {
        attribute diameter = 0.25;
    }

    part def Rover {
        attribute mass = 899.0;

        part battery : Battery;
        part wheels : Wheel[6];

        port power : PowerPort;
        port telemetry : TelemetryPort;
    }

    part def GroundStation {
        port uplink : TelemetryPort;
    }

    part def Mission {
        part rover : Rover;
        part station : GroundStation;

        connect rover.telemetry to station.uplink;
    }

    comment about Wheel /* Diameter is in metres. */
}
