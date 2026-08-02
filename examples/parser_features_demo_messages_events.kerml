/*
 * Parser Features Demo: Messages & Events
 * 
 * Demonstrates message and event keywords:
 * - message keyword (Task 88)
 * - event modifier (Task 89)
 * - ref with body (Task 90)
 * 
 * Based on real stdlib patterns from Flows.sysml
 */

package MessagesAndEvents {
    
    // ===== Message Keyword (Task 88) =====
    
    // Message is a synonym for flow
    abstract message Message {
        feature content: String;
        feature timestamp: Real;
    }
    
    // Message usage
    abstract message messages: Message[0..*] nonunique;
    
    // ===== Event Modifier (Task 89) =====
    
    // Event modifier marks occurrences/features as events
    feature EventBasedFlow {
        // Event occurrence as parameter
        in event occurrence sourceEvent [1];
        in event occurrence targetEvent [0..1];
    }
    
    // ===== Ref with Body (Task 90) =====
    
    ref payload [0..*] {
        doc
        /* Reference feature with body members */
        
        feature dataType: String;
        feature size: Integer;
    }
    
    // ===== Combined Pattern =====
    
    feature MessageChannel {
        // Message flows
        message incoming: Message[*] nonunique;
        message outgoing: Message[*] nonunique;
        
        // Event-driven
        in event occurrence events [*];
        
        // Ref with body
        ref config [1] {
            feature maxSize: Integer;
            feature timeout: Real;
        }
    }
}
