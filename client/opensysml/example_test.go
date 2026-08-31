package opensysml_test

import (
	"context"
	"fmt"
	"log"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// Example parses a model, evaluates an expression against an object, and
// instantiates a definition — all in process.
func Example() {
	client, err := opensysml.New()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()

	model, err := client.ParseSource(ctx, `package Demo {
		part def Vehicle {
			attribute mass = 1500.0;
		}
		part sedan : Vehicle {
			attribute :>> mass = 1800.0;
		}
	}`)
	if err != nil {
		log.Fatal(err)
	}

	mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sedan mass: %v\n", mass)

	instantiation, err := client.Instantiate(ctx, model, "Demo::Vehicle")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("instance of: %s\n", instantiation.Root.TypeSymbolID)
	// Output:
	// sedan mass: 1800
	// instance of: Demo::Vehicle
}

// ExampleClient_ExecuteAction runs an action with an input bound and reads the
// output parameter it produced.
func ExampleClient_ExecuteAction() {
	client, err := opensysml.New()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()

	model, err := client.ParseSource(ctx, `package Demo {
		private import ScalarValues::*;
		action addFive {
			attribute result : Integer = 0;
			first start;
			action inner {
				assign result := result + 5;
			}
			done;
			succession first start then inner;
			succession first inner then done;
		}
	}`)
	if err != nil {
		log.Fatal(err)
	}

	run, err := client.ExecuteAction(ctx, model, "Demo::addFive",
		map[string]opensysml.Value{"result": opensysml.Int(10)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("result: %v\n", run.Outputs["result"])
	// Output:
	// result: 15
}

// ExampleClient_VerifyConstraint verifies a constraint against one part's own
// values. A verdict of false is an answer about the model, not an error.
func ExampleClient_VerifyConstraint() {
	client, err := opensysml.New()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()

	model, err := client.ParseSource(ctx, `package Demo {
		part def Vehicle {
			attribute mass = 1500.0;
			constraint light {
				mass < 1300.0
			}
		}
		part sedan : Vehicle {
			attribute :>> mass = 1200.0;
		}
	}`)
	if err != nil {
		log.Fatal(err)
	}

	for _, subject := range []string{"Demo::sedan", ""} {
		var opts []opensysml.VerifyOption
		if subject != "" {
			opts = append(opts, opensysml.Against(subject))
		}
		verification, err := client.VerifyConstraint(ctx, model, "Demo::Vehicle::light", opts...)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("holds: %t\n", verification.Verdict.Holds)
	}
	// Output:
	// holds: true
	// holds: false
}
