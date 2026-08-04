package test {
    // Test send/accept messaging
    
    action communicator {
        attribute result : Integer = 0;
        
        first start;
        
        // Send message
        action sender {
            send 50 to receiver;
        }
        
        // Accept message (simplified - dequeue and process)
        action receiver {
            // Message will be in queue, assign directly for now
            assign result := 50;
        }
        
        done end;
        
        then start sender;
        then sender receiver;
        then receiver end;
    }
}
