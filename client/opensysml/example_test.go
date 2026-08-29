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
