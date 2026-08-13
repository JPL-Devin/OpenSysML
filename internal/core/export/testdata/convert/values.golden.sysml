package Values {
    calc def Total {
        in attribute x : Real;
        in attribute y : Real;
        return attribute result : Real;
    }
    part def Sized {
        attribute size : Real = 4.5;
        attribute label : String = "front";
        attribute total : Real = 1.0 + 2.0;
    }
}
