// REPL Behavioral Commands Demo
//
// This file demonstrates the new REPL commands for evaluating
// behavioral elements: calculations, constraints, and requirements.
//
// Try these commands in the REPL:
//   %load examples/repl-behavioral-demo.sysml
//   %calc distance 3 4
//   %calc volume 2 3 5
//   %calc factorial 5
//   %constraint PositiveArea
//   %constraint ValidSpeed
//   %constraint InvalidSpeed
//   %requirement SafetyReq
//   %requirement PerformanceReq
//   %requirement ViolatedReq

// ============================================================================
// CALCULATIONS
// ============================================================================

// Calculate distance using Pythagorean theorem
calc distance {
    in x;
    in y;
    x * x + y * y  // simplified: should use sqrt
}

// Calculate volume of a box
calc volume {
    in length;
    in width;
    in height;
    length * width * height
}

// Calculate factorial (iterative approximation using product)
calc factorial {
    in n;
    n * (n - 1)  // simplified: just n * (n-1) for demo
}

// Convert temperature from Celsius to Fahrenheit
calc celsiusToFahrenheit {
    in celsius;
    celsius * 9 / 5 + 32
}

// Calculate area of rectangle
calc rectangleArea {
    in length;
    in width;
    length * width
}

// ============================================================================
// CONSTRAINTS
// ============================================================================

// Constraint: area must be positive
constraint PositiveArea {
    10 * 5 > 0
}

// Constraint: speed must be within valid range
constraint ValidSpeed {
    65 >= 0 and 65 <= 120
}

// Constraint: this will fail - invalid speed
constraint InvalidSpeed {
    150 <= 120
}

// Constraint: both dimensions positive
constraint AssumePositive {
    10 > 0 and 5 > 0
}

// Constraint: negated assertion
constraint NotNegative {
    not (50 < 0)
}

// ============================================================================
// REQUIREMENTS
// ============================================================================

// Safety requirement: braking distance must be adequate
requirement SafetyReq {
    // Assume reasonable speed
    assume constraint { 65 > 0 }
    assume constraint { 65 < 200 }

    // Require adequate braking distance
    require constraint { 100 > 50 }
}

// Performance requirement: acceleration must meet target
requirement PerformanceReq {
    // Assume valid conditions
    assume constraint { 100 > 0 }

    // Require performance threshold
    require constraint { 95 >= 90 }
}

// This requirement will fail
requirement ViolatedReq {
    // Assumptions pass
    assume constraint { 50 > 0 }

    // But requirement fails
    require constraint { 30 > 100 }
}

// Complex requirement with multiple conditions
requirement ComplexReq {
    assume constraint { 100 >= 0 }
    assume constraint { 100 <= 200 }

    require constraint { 150 > 100 }
    require constraint { 150 < 200 }
}

// ============================================================================
// USAGE EXAMPLES
// ============================================================================

// To test these in the REPL:
//
// 1. Start the REPL:
//    $ sysml repl
//
// 2. Load this file:
//    %load examples/repl-behavioral-demo.sysml
//
// 3. Invoke calculations:
//    %calc distance 3 4              // Returns: 25 (3² + 4²)
//    %calc volume 2 3 5              // Returns: 30
//    %calc factorial 5               // Returns: 20 (simplified)
//    %calc celsiusToFahrenheit 100   // Returns: 212
//    %calc rectangleArea 10 5        // Returns: 50
//
// 4. Evaluate constraints:
//    %constraint PositiveArea        // ✓ passed
//    %constraint ValidSpeed          // ✓ passed
//    %constraint InvalidSpeed        // ✗ failed (expected)
//    %constraint AssumePositive      // ✓ passed
//    %constraint NotNegative         // ✓ passed
//
// 5. Evaluate requirements:
//    %requirement SafetyReq          // ✓ satisfied
//    %requirement PerformanceReq     // ✓ satisfied
//    %requirement ViolatedReq        // ✗ failed (expected)
//    %requirement ComplexReq         // ✓ satisfied
//
// Note: Arguments are parsed as literal expressions. You can use:
//   - Integer literals: 10, -5, 100
//   - Arithmetic: 3 + 4, 10 * 2
//   - Parentheses: (5 + 3) * 2
