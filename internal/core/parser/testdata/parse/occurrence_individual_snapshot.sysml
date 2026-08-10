package IndividualsAndSnapshots {
    occurrence def Flight;
    part def Vehicle;
    individual def TestSystem :> Vehicle;

    // `individual` on a usage: OccurrenceUsage::isIndividual (SysML v2 §8.3.9.11).
    individual testSystem : TestSystem;

    // `snapshot` on a usage: OccurrenceUsage::portionKind = snapshot.
    snapshot occurrence takeoff : Flight;
    snapshot part vehicleAtTakeoff : Vehicle;

    part def System {
        individual testVehicle : TestSystem;

        action collectData {
            in individual subjectVehicle : TestSystem;
            in snapshot vehicleState : Flight;
        }
    }
}
