// =============================================================================
// Semantic Layer Feature Demo
// Demonstrates runtime operators, feature chains, and validation
// =============================================================================

package SemanticLayerDemo {

    // =========================================================================
    // Track 1: Runtime Operators
    // =========================================================================
    
    // Equality operators
    attribute intEquality = 42 == 42;          // true
    attribute intInequality = 42 != 43;        // true
    attribute realEquality = 3.14 == 3.14;     // true
    attribute nullEquality = null == null;     // true
    attribute crossKindCheck = 42 == null;     // false
    
    // Logical operators with short-circuit evaluation
    attribute andTrue = (5 > 3) & (10 < 20);   // true
    attribute andFalse = (5 < 3) & (10 < 20);  // false
    attribute orTrue = (5 > 3) | (10 > 20);    // true
    attribute orFalse = (5 < 3) | (10 > 20);   // false
    attribute notTrue = not (5 < 3);           // true
    attribute notFalse = not (5 > 3);          // false
    attribute doubleNot = not (not true);      // true
    
    // Arithmetic negation
    attribute negInt = -42;                    // -42
    attribute negReal = -3.14;                 // -3.14
    attribute doubleNeg = -(-5);               // 5
    attribute negExpr = -(3 + 2);              // -5
    
    // Qualified name lookup - multi-part names
    namespace MathConstants {
        attribute pi = 3.14159;
        attribute e = 2.71828;
        
        namespace Derived {
            attribute twoPi = 6.28318;
        }
    }
    
    attribute usePi = MathConstants::pi;                 // 3.14159
    attribute useE = MathConstants::e;                   // 2.71828
    attribute nestedLookup = MathConstants::Derived::twoPi;  // 6.28318
    
    // Combined operators - realistic expressions
    attribute expr1 = (42 == 42) & (100 != 99);          // true
    attribute expr2 = MathConstants::pi > 3;             // true
    attribute expr3 = -(MathConstants::e) < 0;           // true
    attribute expr4 = (10 > 5) | (3 == 2);              // true
    attribute expr5 = not ((5 < 3) & (10 > 20));        // true

    // =========================================================================
    // Track 2: Feature Chain Resolution
    // =========================================================================
    
    // Simple feature chains - member access
    part def Container {
        attribute value = 0;
        attribute name = "default";
    }
    
    part myContainer : Container;
    
    // Simple chain: object.member
    attribute simpleValue = myContainer.value;          // 0
    attribute simpleName = myContainer.name;            // "default"
    
    // Nested chains
    part def Nested {
        part inner : Container;
        attribute id = 0;
    }
    
    part myNested : Nested;
    
    attribute nestedValue = myNested.inner.value;       // 0
    attribute nestedId = myNested.id;                   // 0

    // =========================================================================
    // Track 3: Semantic Validation
    // =========================================================================
    
    // Typing conformance - valid specialization
    attribute def SpeedType;
    
    part def Vehicle {
        attribute speed : SpeedType;
    }
    
    part def Car specializes Vehicle {
        // Valid: Car.speed inherits Vehicle.speed type
    }
    
    // Valid subsetting - Car specializes Vehicle
    part allVehicles : Vehicle[*];
    part myCar : Car subsets allVehicles;  // OK: Car conforms to Vehicle

    // =========================================================================
    // Integration Example
    // =========================================================================
    
    namespace Integration {
        part def Sensor {
            attribute reading = 0.0;
            attribute threshold = 100.0;
        }
        
        part tempSensor : Sensor;
        part pressureSensor : Sensor;
        
        // Feature chains to access nested values
        attribute tempValue = tempSensor.reading;       // 0.0
        attribute tempLimit = tempSensor.threshold;     // 100.0
        attribute pressureValue = pressureSensor.reading;   // 0.0
        attribute pressureLimit = pressureSensor.threshold; // 100.0
        
        // Operators on chain results
        attribute isHighTemp = tempValue > 90.0;           // false
        attribute isNormalPressure = pressureValue < 200.0;  // true
        
        // Combined logic
        attribute systemOK = (not isHighTemp) & isNormalPressure;  // true
    }
    
    // =========================================================================
    // Edge Cases
    // =========================================================================
    
    // Negation of comparison results
    attribute notEqual = not (42 == 43);                // true
    
    // Chained comparisons with negation
    attribute complex1 = -(5) < 0;                      // true
    attribute complex2 = -(-10) == 10;                  // true
    
    // Multiple logical operators
    attribute multiLogic = (true & false) | (true & true);  // true
    
    // Parenthesized expressions
    attribute grouped = ((5 > 3) & (10 < 20)) | (1 == 2);  // true
}
