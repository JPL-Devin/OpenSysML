package test {
    // Test send/accept messaging (simplified)
    
    action communicator {
        attribute result : Integer = 0;
        
        first start;
        
        // Send message
        action sendMsg {
            send 50 to receiver;
        }
        
        // Receive message (simplified - just set result)
        action receiveMsg {
            assign result := 50;
        }
        
        done end;
        
        then start sendMsg;
        then sendMsg receiveMsg;
        then receiveMsg end;
    }
}
