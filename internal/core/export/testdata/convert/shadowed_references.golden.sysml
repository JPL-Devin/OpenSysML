package Shadowing {
    attribute def Field;
    attribute 'packet data field' : Field;
    part def Packet {
        attribute 'data field' : Field;
        part payload;
    }
    part def 'Thermal Packet' specializes Packet {
        attribute 'packet data field' redefines 'packet data field';
        attribute 'data field' redefines 'data field' {
            attribute 'data field' : Field;
        }
    }
    part payload;
    part def Bus {
        part payload subsets Shadowing::payload;
        part cargo subsets Shadowing::payload;
        ref part carried references payload;
        part def Packet;
        part wrapped : Shadowing::Packet;
        part raw : Packet;
        connection link connect wrapped.payload to Shadowing::payload;
    }
    part def Frame {
        part def Frame;
        attribute 'packet data field' : Shadowing::Field;
        ref part outer : Shadowing::Frame;
        ref part inner : Frame;
        part def Field;
    }
}
