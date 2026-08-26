package ChoiceTest {
    state TemperatureControl {
        attribute temperature : Integer = 25;
        attribute mode : String = "unknown";
        
        initial start;
        state idle;
        choice tempCheck;
        state heating;
        state cooling;
        state normal;
        final done;
        
        succession first start then idle;
        transition idle to tempCheck;
        transition tempCheck to heating if temperature < 18;
        transition tempCheck to cooling if temperature > 24;
        transition tempCheck to normal;  // else branch
        
        entry heating { assign mode := "heating"; }
        entry cooling { assign mode := "cooling"; }
        entry normal { assign mode := "normal"; }
        
        transition heating to done;
        transition cooling to done;
        transition normal to done;
    }
}
