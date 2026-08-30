package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import Views::*;

	part def Subsystem {
		attribute mass : Real;
	}

	part def 'Assembly *frame*';

	part telescope : 'Assembly *frame*' {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
		}
		part segmentControl : Subsystem {
			attribute redefines mass = 20.0;
		}
		part mount : Subsystem {
			attribute redefines mass = 15.0;
		}
		part 'baffle|shroud *tricky*' : Subsystem {
			attribute redefines mass = 1.5;
		}
	}

	port def DataPort;

	part def Camera {
		port output : DataPort;
	}

	part def Recorder {
		port input : DataPort;
	}

	part imagingChain {
		part camera : Camera;
		part recorder : Recorder;
		connection link connect camera.output to recorder.input;
	}

	state def ObservatoryStates {
		entry; then idle;
		state idle;
		state observing;
		transition first idle then observing;
		transition first observing then idle;
	}

	state operatingStates : ObservatoryStates;

	view interconnectView {
		expose imagingChain;
		render asInterconnectionDiagram;
	}

	calc def Subsystems :> Query {
		in root : Element;
		WhereType(
			source = Descendants(source = root, maxDepth = 3),
			type = "PartUsage"
		)
	}

	calc def SubsystemTable :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Subsystems(root = root),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "mass")
		)
	}

	calc def HeavySubsystemNames :> Query {
		in root : Element;
		in threshold : String;
		Project(
			source = OrderBy(
				source = WhereFeature(
					source = Subsystems(root = root),
					'feature' = "mass",
					operator = ">=",
					value = threshold
				),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name")
		)
	}

	calc def MissingSubsystems :> Query {
		in root : Element;
		Project(
			source = WhereName(
				source = Subsystems(root = root),
				operator = "=",
				value = "Nonexistent"
			),
			properties = ("name", "mass")
		)
	}

	calc def DeclaredTypes :> Query {
		in root : Element;
		RelatedElements(
			source = root,
			relationshipKind = "typing",
			direction = "outgoing",
			maxDepth = 1
		)
	}

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			attribute redefines text = "Mass rollup for the telescope assembly. Masses are in kg | not #grams, *not* _lbs_, `raw`, <b>&plain</b>.";
		}

		part breakdown : Section {
			attribute redefines title = "Subsystem Masses | by *name*";

			part masses : Table {
				attribute redefines caption = "All subsystems by mass";
				calc rows : SubsystemTable {
					in root = telescope;
				}
			}

			part heavy : Section {
				attribute redefines title = "Heavy Subsystems";

				part summary : Paragraph {
					calc names : HeavySubsystemNames {
						in root = telescope;
						in threshold = "10";
					}
				}

				part heavyItems : List {
					attribute redefines style = "number";
					calc items : HeavySubsystemNames {
						in root = telescope;
						in threshold = "10";
					}
				}
			}

			part missing : Section {
				attribute redefines title = "Missing Subsystems";

				part missingTable : Table {
					calc rows : MissingSubsystems {
						in root = telescope;
					}
				}

				part missingItems : List {
					calc items : MissingSubsystems {
						in root = telescope;
					}
				}
			}
		}

		part diagrams : Section {
			attribute redefines title = "Diagrams";

			part imaging : Diagram {
				attribute redefines caption = "Imaging chain interconnection";
				ref redefines source = interconnectView;
			}

			part states : Diagram {
				attribute redefines caption = "Observatory states, left to right";
				attribute redefines kind = "state";
				attribute redefines direction = "LR";
				ref redefines source = operatingStates;
			}
		}

		part typing : Section {
			attribute redefines title = "Declared Types";

			part note : Paragraph {
				attribute redefines text = "The declared type of the telescope, by relationship traversal.";
			}

			part typeTable : Table {
				attribute redefines caption = "Type of telescope";
				calc rows : DeclaredTypes {
					in root = telescope;
				}
			}

			part typeItems : List {
				calc items : DeclaredTypes {
					in root = telescope;
				}
			}
		}
	}
}
