package test {
    // Test type coercion: Integer + Real → Real
    calc testCoercion {
        in intVal : Integer;
        in realVal : Real;
        return intVal + realVal;  // Integer coerced to Real
    }
}
