/*
 * OOSEM demo: a small Earth-observation mission worked through the six OOSEM
 * activities with the OpenSysML `OOSEM` library. Each package is stamped with
 * the activity it belongs to; each artefact carries the method keyword that
 * types it and adds it to the corresponding base usage.
 *
 *   ./bin/sysml examples/oosem-demo/oosem-demo.sysml -validate
 */
package 'Wildfire Watch Mission' {
	private import OOSEM::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	@OOSEMPackage { kind = OOSEMPackageKind::enterpriseModel; }
	package 'Enterprise Model' {
		#enterprise part def 'Wildfire Response Enterprise' {
			#stakeholderRole part incidentCommand : 'Incident Command';
			#stakeholderRole part forestService : 'Forest Service';
			#externalSystem part groundStation : 'Ground Station Network';
		}

		#stakeholderRole part def 'Incident Command';
		#stakeholderRole part def 'Forest Service';
		#externalSystem part def 'Ground Station Network';

		#asIs part def 'Aerial Patrol Enterprise' :> 'Wildfire Response Enterprise' {
			doc /* Today: crewed aircraft patrol on a schedule; ignitions are found hours late. */
		}

		#toBe part def 'Satellite Watch Enterprise' :> 'Wildfire Response Enterprise' {
			doc /* Proposed: a small satellite revisits every fire-prone region within hours. */
			#system part firewatch : 'System Requirements'::'FireWatch Satellite';
		}

		#causalAnalysis analysis def 'Late Detection Analysis' {
			subject enterprise : 'Aerial Patrol Enterprise';
			doc /* Traces late ignition reports to sparse patrol coverage rather than to reporting delay. */
		}

		#enterpriseUseCase use case def 'Contain Wildfire' {
			subject enterprise : 'Wildfire Response Enterprise';
			actor incidentCommand : 'Incident Command';
			objective { doc /* Contain a new ignition before it reaches 10 hectares. */ }
		}
	}

	@OOSEMPackage { kind = OOSEMPackageKind::stakeholderNeeds; }
	package 'Stakeholder Needs' {
		private import 'Enterprise Model'::*;

		#stakeholderNeed requirement def 'Early Warning Need' {
			doc /* Incident command needs to learn of a new ignition soon enough to contain it. */
			subject enterprise : 'Wildfire Response Enterprise';
			stakeholder incidentCommand : 'Incident Command';
			#moe attribute detectionLatency : DurationValue;
		}

		#stakeholderNeed requirement earlyWarning : 'Early Warning Need';
	}

	@OOSEMPackage { kind = OOSEMPackageKind::missionRequirements; }
	package 'Mission Requirements' {
		private import 'Enterprise Model'::*;
		private import 'Stakeholder Needs'::*;

		#missionRequirement requirement def 'Detection Latency' {
			doc /* The enterprise shall report a new ignition within 3 hours of its start. */
			subject enterprise : 'Satellite Watch Enterprise';
			#moe attribute maxLatency : DurationValue = 3 [h];
		}

		#missionRequirement requirement detectionLatency : 'Detection Latency';

		#derivation connection {
			end #original ::> earlyWarning;
			end #derive ::> detectionLatency;
		}
	}

	@OOSEMPackage { kind = OOSEMPackageKind::systemRequirements; }
	package 'System Requirements' {
		private import 'Enterprise Model'::*;
		private import 'Mission Requirements'::*;

		#system part def 'FireWatch Satellite';
		#user part def 'Mission Operator';
		#environment part def 'Low Earth Orbit' {
			attribute altitude : LengthValue = 550 [km];
		}
		#ioEntity item def 'Fire Alert' {
			attribute latitude : Real;
			attribute longitude : Real;
			attribute confidence : Real;
		}
		#ioEntity item def 'Thermal Image';
		#store item def 'Alert Archive';

		#systemContext part def 'FireWatch Context' {
			part :>> systemOfInterest : 'FireWatch Satellite';
			part groundStation : 'Ground Station Network' :>> externalSystems;
			part operator : 'Mission Operator' :>> users;
			part :>> environment : 'Low Earth Orbit';
		}

		#systemUseCase use case def 'Detect Ignition' {
			subject context : 'FireWatch Context';
			actor groundStation : 'Ground Station Network';
			objective { doc /* Image a fire-prone region and downlink an alert for any new hot spot. */ }
		}

		#systemRequirement requirement def 'Revisit Time' {
			doc /* The satellite shall revisit each fire-prone region at least once every 2 hours. */
			subject satellite : 'FireWatch Satellite';
			#mop attribute revisit : DurationValue = 2 [h];
		}

		#systemRequirement requirement def 'Alert Downlink' {
			doc /* The satellite shall downlink a Fire Alert within 15 minutes of imaging a hot spot. */
			subject satellite : 'FireWatch Satellite';
			#mop attribute downlinkLatency : DurationValue = 15 [min];
		}

		#systemRequirement requirement revisitTime : 'Revisit Time';
		#systemRequirement requirement alertDownlink : 'Alert Downlink';

		#derivation connection {
			end #original ::> detectionLatency;
			end #derive ::> revisitTime;
			end #derive ::> alertDownlink;
		}
	}

	@OOSEMPackage { kind = OOSEMPackageKind::logicalArchitecture; }
	package 'Logical Architecture' {
		private import 'System Requirements'::*;

		#logical part def 'Thermal Sensing';
		#logical part def 'Hot Spot Detection';
		#logical part def 'Alert Downlink Service';

		part def 'FireWatch Logical' :> 'FireWatch Satellite' {
			#logical part sensing : 'Thermal Sensing';
			#logical part detection : 'Hot Spot Detection';
			#logical part downlink : 'Alert Downlink Service';
		}

		#logicalScenario action def 'Detect Ignition Scenario' {
			action image : 'Capture Image';
			then action detect : 'Find Hot Spots';
			then action transmit : 'Send Alert';
		}

		action def 'Capture Image' { out image : 'Thermal Image'; }
		action def 'Find Hot Spots' { in image : 'Thermal Image'; out alert : 'Fire Alert'; }
		action def 'Send Alert' { in alert : 'Fire Alert'; }

		#componentRequirement requirement def 'Detection Time' {
			doc /* Hot spot detection shall complete within 5 minutes of image capture. */
			subject detection : 'Hot Spot Detection';
			#mop attribute detectionTime : DurationValue = 5 [min];
		}

		#componentRequirement requirement detectionTime : 'Detection Time';

		#derivation connection {
			end #original ::> alertDownlink;
			end #derive ::> detectionTime;
		}
	}

	@OOSEMPackage { kind = OOSEMPackageKind::physicalArchitecture; }
	package 'Physical Architecture' {
		private import 'Logical Architecture'::*;

		#hardware part def 'Thermal Infrared Camera';
		#hardware part def 'Flight Computer';
		#software part def 'Detection Software';
		#software part def 'Comms Software';
		#hardware part def 'X-Band Transmitter';
		#operationalProcedure part def 'Alert Triage Procedure';
		#data item def 'Region Watch List';

		#node part def 'Space Segment';
		#node part def 'Ground Segment';

		part def 'FireWatch Physical' :> 'System Requirements'::'FireWatch Satellite' {
			#node part space : 'Space Segment' {
				#hardware part camera : 'Thermal Infrared Camera';
				#hardware part computer : 'Flight Computer' {
					#software part detector : 'Detection Software';
					#software part comms : 'Comms Software';
					#data item watchList : 'Region Watch List';
				}
				#hardware part transmitter : 'X-Band Transmitter';
			}
			#node part ground : 'Ground Segment' {
				#operationalProcedure part triage : 'Alert Triage Procedure';
			}
		}
	}

	@OOSEMPackage { kind = OOSEMPackageKind::tradeStudies; }
	package 'Trade Studies' {
		private import 'Physical Architecture'::*;

		attribute def CameraEvaluation :> Real;

		#hardware part def 'Wide Field Camera' :> 'Thermal Infrared Camera';
		#hardware part def 'Narrow Field Camera' :> 'Thermal Infrared Camera';

		part wideField : 'Wide Field Camera';
		part narrowField : 'Narrow Field Camera';
	}
}
