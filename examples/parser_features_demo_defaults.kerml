/*
 * Parser Features Demo - Default Values
 * 
 * Demonstrates 'default' keyword as alternative to '=' for value assignment.
 * This is syntactic sugar that provides clearer intent for default values.
 */

package demo_defaults {
	
	// Default keyword vs equals sign
	feature comparisonExamples {
		doc /* Both syntaxes are equivalent */
		
		// Traditional assignment with =
		feature timeout: Integer[1] = 30;
		
		// Default keyword (clearer intent)
		feature retryCount: Integer[1] default 3;
		
		// Works with multiplicity
		feature maxConnections: Integer[0..1] default 100;
	}
	
	// Configuration with defaults
	feature systemConfig {
		doc /* Configuration parameters with sensible defaults */
		
		feature networking {
			feature host: String[1] default "localhost";
			feature port: Integer[1] default 8080;
			feature useSSL: Boolean[1] default false;
		}
		
		feature logging {
			feature level: String[1] default "INFO";
			feature maxFileSize: Integer[1] default 10485760; // 10MB
			feature retentionDays: Integer[1] default 7;
		}
		
		feature performance {
			feature threadPoolSize: Integer[1] default 4;
			feature queueCapacity: Integer[1] default 1000;
			feature timeout: Real[1] default 30.0;
		}
	}
	
	// Anonymous features with default (from Task 51 fix)
	port interface APIEndpoint {
		doc /* 
		 * Port parameters with defaults.
		 * Note: Fully anonymous features (just multiplicity + default) 
		 * are an edge case pattern.
		 */
		
		feature enabled: Boolean[1] default true;
		feature supportedMethods: String[1..*] default "GET";
	}
	
	// Action parameters with defaults
	action processRequest {
		in feature url: String[1];
		in feature method: String[1] default "GET";
		in feature timeout: Integer[1] default 30;
		in feature retries: Integer[1] default 3;
		
		doc /* Action with default parameter values */
	}
	
	// Real-world example: HTTP client configuration
	item HttpClient {
		feature config {
			feature baseURL: String[1];
			feature timeout: Real[1] default 30.0;
			feature followRedirects: Boolean[1] default true;
			feature maxRedirects: Integer[1] default 5;
			feature userAgent: String[1] default "Systemica/1.0";
			feature compression: Boolean[1] default true;
			
			feature headers {
				feature contentType: String[1] default "application/json";
				feature accept: String[1] default "application/json";
			}
		}
	}
}
