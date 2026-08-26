package ChoiceTest {
    state TemperatureControl {
        attribute temperature : Integer = 25;
        attribute mode : String = "unknown";
        
        entry; then start;
        state start;
        state idle;
        choice tempCheck;
        state heating;
        state cooling;
        state normal;
        
        succession first start then idle;
        transition first idle then tempCheck;
        transition first tempCheck if temperature < 18 then heating;
        transition first tempCheck if temperature > 24 then cooling;
        transition first tempCheck then normal;  // else branch
        
        entry heating { assign mode := "heating"; }
        entry cooling { assign mode := "cooling"; }
        entry normal { assign mode := "normal"; }
        
        transition first heating then done;
        transition first cooling then done;
        transition first normal then done;
    }
}
