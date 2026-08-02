// =============================================================================
// Semantic Layer Feature Demo
// Demonstrates runtime operators, resolution, and validation
// =============================================================================

package SemanticLayerDemo {

    // =========================================================================
    // Track 1: Runtime Operators
    // =========================================================================
    
    // Equality operators
    attribute equalityDemo {
        attribute intEquality = 42 == 42;          // true
        attribute intInequality = 42 != 43;        // true
        attribute realEquality = 3.14 == 3.14;     // true
        attribute nullEquality = null == null;     // true
        attribute crossKindCheck = 42 == null;     // false
    }
    
    // Logical operators with short-circuit evaluation
    attribute logicalDemo {
        attribute andTrue = (5 > 3) & (10 < 20);   // true
        attribute andFalse = (5 < 3) & (10 < 20);  // false (short-circuits)
        attribute orTrue = (5 > 3) | (10 > 20);    // true (short-circuits)
        attribute orFalse = (5 < 3) | (10 > 20);   // false
        attribute notTrue = not (5 < 3);           // true
        attribute notFalse = not (5 > 3);          // false
        attribute doubleNot = not (not true);      // true
    }
    
    // Arithmetic negation
    attribute negationDemo {
        attribute negInt = -42;                    // -42
        attribute negReal = -3.14;                 // -3.14
        attribute doubleNeg = -(-5);               // 5
        attribute negExpr = -(3 + 2);              // -5
    }
    
    // Qualified name lookup - multi-part names
    namespace MathConstants {
        attribute pi = 3.14159;
        attribute e = 2.71828;
        
        namespace Derived {
            attribute twoPi = 6.28318;
        }
    }
    
    attribute qualifiedDemo {
        attribute usePi = MathConstants::pi;                 // 3.14159
        attribute useE = MathConstants::e;                   // 2.71828
        attribute nestedLookup = MathConstants::Derived::twoPi;  // 6.28318
    }
    
    // Combined operators - realistic expressions
    attribute combinedDemo {
        attribute expr1 = (42 == 42) & (100 != 99);          // true
        attribute expr2 = MathConstants::pi > 3;             // true
        attribute expr3 = -(MathConstants::e) < 0;           // true
        attribute expr4 = (10 > 5) | (3 == 2);              // true
        attribute expr5 = not ((5 < 3) & (10 > 20));        // true
    }

    // =========================================================================
    // Track 3: Feature Chain Resolution
    // =========================================================================
    
    // Simple feature chains - member access
    part def Container {
        attribute value;
        attribute name;
    }
    
    part def Nested {
        part inner : Container;
        attribute id;
    }
    
    attribute chainDemo {
        part myContainer : Container {
            attribute redefines value = 42;
            attribute redefines name = "test";
        }
        
        part myNested : Nested {
            attribute redefines id = 1;
            part redefines inner : Container {
                attribute redefines value = 100;
            }
        }
        
        // Simple chain: object.member
        attribute simpleValue = myContainer.value;          // 42
        attribute simpleName = myContainer.name;            // "test"
        
        // Nested chains: object.member.member
        attribute nestedValue = myNested.inner.value;       // 100
        attribute nestedId = myNested.id;                   // 1
    }
    
    // Feature chains in relationships - subsetting
    part def Point {
        attribute x;
        attribute y;
    }
    
    part def Shape {
        part origin : Point;
    }
    
    part def Rectangle specializes Shape {
        // Feature chain subsetting (origin inherited from Shape)
        part topLeft : Point;
        // Note: Direct subsetting of inherited members requires qualified name
    }
    
    // Feature chains in redefinitions
    part def Component {
        part config : Container;
    }
    
    part def AdvancedComponent specializes Component {
        // Redefines with feature chain
        part redefines config : Container {
            attribute redefines value = 200;
        }
    }

    // =========================================================================
    // Track 4: Semantic Validation
    // =========================================================================
    
    // Typing conformance - subsetting must respect type hierarchy
    attribute def SpeedType;
    
    part def Vehicle {
        attribute speed : SpeedType[1..2];
    }
    
    part def Car specializes Vehicle {
        // Valid: Car.speed type conforms to Vehicle.speed
        // Multiplicity [1..1] is within [1..2]
        attribute redefines speed : SpeedType[1..1];
    }
    
    attribute validationDemo {
        attribute allVehicles : Vehicle[*];
        
        // Valid: Car conforms to Vehicle (Car specializes Vehicle)
        attribute myCar : Car subsets allVehicles;
    }
    
    // Redefinition validation - must follow SysML rules
    part def Engine {
        attribute power[100..500];
        attribute efficiency[0..1];
    }
    
    part def ElectricEngine specializes Engine {
        // Both redefinitions valid:
        // - Inherited from Engine ✓
        // - Multiplicity bounds valid ✓
        attribute redefines power[200..400];       // Narrows range
        attribute redefines efficiency[0.8..0.95];    // Narrows range
    }
    
    part def HybridEngine specializes Engine {
        // Valid redefinitions maintaining constraints
        attribute redefines power[150..450];
        attribute redefines efficiency[0.75..0.98];
    }

    // =========================================================================
    // Comprehensive Integration Example
    // =========================================================================
    
    // Realistic scenario combining all features
    namespace VehicleSystem {
        
        part def Sensor {
            attribute reading;
            attribute threshold;
        }
        
        part def MonitoringSystem {
            part tempSensor : Sensor;
            part pressureSensor : Sensor;
        }
        
        part def AdvancedMonitoring specializes MonitoringSystem {
            // Redefinitions with validation
            part redefines tempSensor : Sensor {
                attribute redefines threshold = 100.0;
            }
            
            part redefines pressureSensor : Sensor {
                attribute redefines threshold = 50.0;
            }
        }
        
        attribute analysis {
            part system : AdvancedMonitoring;
            
            // Feature chains to access nested values
            attribute tempThreshold = system.tempSensor.threshold;       // 100.0
            attribute pressureThreshold = system.pressureSensor.threshold; // 50.0
            
            // Operators on chain results
            attribute isHighTemp = tempThreshold > 90.0;                // true
            attribute isLowPressure = pressureThreshold < 60.0;         // true
            
            // Combined logic
            attribute alertNeeded = isHighTemp & isLowPressure;         // true
            attribute systemOK = not alertNeeded;                       // false
            
            // Qualified names with operators
            attribute compared = tempThreshold == MathConstants::Derived::twoPi; // false
        }
    }
    
    // =========================================================================
    // Edge Cases and Advanced Usage
    // =========================================================================
    
    attribute edgeCases {
        // Negation of comparison results
        attribute notEqual = not (42 == 43);                    // true
        
        // Chained comparisons with negation
        attribute complex1 = -(5) < 0;                          // true
        attribute complex2 = -(-10) == 10;                      // true
        
        // Multiple logical operators
        attribute multiLogic = (true & false) | (true & true);  // true
        
        // Parenthesized expressions
        attribute grouped = ((5 > 3) & (10 < 20)) | (1 == 2);  // true
    }
}
