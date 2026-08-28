#!/usr/bin/env python3
"""Drive the rover model from Python: what it is, what it does, what holds.

Run it from the repository root, with the `opensysml` client installed and a
`sysml-grpc` service available (see docs/guide/09-python.md):

    python examples/rover-demo/rover_demo.py
"""

import pathlib

import opensysml

MODEL = pathlib.Path(__file__).with_name("rover.sysml")


def main() -> None:
    model = opensysml.load(str(MODEL), strict=True)

    print("== the rover as an object")
    rover = model.instantiate("Rover::curiosity")
    print(f"mass          {rover.mass} kg")
    print(f"drive power   {rover.mobility.drivePower} W at {rover.mobility.speed} m/s")
    print(f"wheels        {len(rover.mobility.wheels)}")
    print(f"battery       {rover.battery.charge} of {rover.battery.capacity} Wh")

    print("\n== planning a traverse")
    metres = model.calc("RoverBehavior::Traverse", arguments=[120.0, 90.0]).value
    energy = model.calc(
        "RoverBehavior::energyForDrive",
        arguments=[metres, rover.mobility.speed, rover.mobility.drivePower],
    ).value
    print(f"{metres} m costs {energy:.1f} Wh of the {rover.battery.charge} Wh aboard")

    print("\n== running the sol")
    results = model.execute_action("RoverBehavior::SurfaceOps")
    print(f"frames held   {results['framesHeld']}")
    print(f"core frames   {results['coreFrames']} (the sampling branch ran through)")
    print(f"downlinked    {results['downlinked']}")
    print(f"charge left   {results['chargeLeft']} Wh")

    print("\n== running the mode machine")
    run = model.execute_state("RoverBehavior::Modes")
    print(f"states        {' -> '.join(run['states_visited'])}")
    written = run["final_context"]
    print("wrote         " + ", ".join(f"{k}={written[k]}" for k in sorted(written)))

    print("\n== checking the sol budget")
    verdicts = model.verify_satisfaction("RoverSolver::plans")
    for verdict in verdicts:
        verdict.raise_for_error()
        print(verdict)
    print(f"both hold     {all(verdicts)}")

    heavy = model.verify_constraint(
        "Rover::Platform::withinMassBudget", subject="Rover::heavyMockup"
    )
    print(f"heavy mockup  {'within' if heavy else 'over'} the mass budget")


if __name__ == "__main__":
    main()
