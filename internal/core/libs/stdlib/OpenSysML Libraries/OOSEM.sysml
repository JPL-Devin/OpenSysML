library package OOSEM {
	doc
	/*
	 * NON-NORMATIVE OpenSysML vocabulary for the Object-Oriented Systems Engineering Method (OOSEM).
	 * Each OOSEM artefact gets a definition specialising its native SysML type, a base usage, and a
	 * semantic-metadata keyword applying both at once: `#system part sat : Satellite;`.
	 * MOE/MOP, trade studies, derivation and causation reuse the standard domain libraries, re-exported here.
	 */

	private import ScalarValues::String;
	private import Metaobjects::SemanticMetadata;
	private import Parts::Part;
	private import Parts::parts;
	private import Items::Item;
	private import Items::items;
	private import Actions::Action;
	private import Actions::actions;
	private import Requirements::RequirementCheck;
	private import Requirements::requirementChecks;
	private import UseCases::UseCase;
	private import UseCases::useCases;
	private import AnalysisCases::AnalysisCase;
	private import AnalysisCases::analysisCases;

	public import ParametersOfInterestMetadata::*;
	public import TradeStudies::*;
	public import RequirementDerivation::*;
	public import CauseAndEffect::*;

	/* Enterprise and stakeholder needs analysis */

	part def Enterprise :> Part {
		doc /* An Enterprise is the organisation, or system of systems, whose mission the system of interest
		 * serves. OOSEM models the as-is enterprise to find the problem and the to-be enterprise that
		 * includes the system of interest. */
	}

	abstract part enterprises : Enterprise[0..*] nonunique :> parts;

	part def Stakeholder :> Part {
		doc /* A Stakeholder is a person, organisation or external system whose needs, concerns and measures of
		 * effectiveness the enterprise and system must satisfy. */
	}

	abstract part stakeholderParts : Stakeholder[0..*] nonunique :> parts;

	analysis def CausalAnalysis :> AnalysisCase {
		doc /* A CausalAnalysis is the OOSEM analysis of an as-is enterprise that traces the problems it
		 * exhibits to their causes, so the to-be enterprise addresses causes rather than symptoms. The
		 * causes and effects themselves are modelled with the CauseAndEffect library re-exported here. */
	}

	abstract analysis causalAnalyses : CausalAnalysis[0..*] nonunique :> analysisCases;

	use case def EnterpriseUseCase :> UseCase {
		doc /* An EnterpriseUseCase describes how the enterprise as a whole achieves a goal for its
		 * stakeholders. Its subject is the enterprise; the system of interest is at most one of its
		 * participants. */
	}

	abstract use case enterpriseUseCases : EnterpriseUseCase[0..*] nonunique :> useCases;

	/* Requirements, by the enterprise level they are stated at. */

	requirement def StakeholderNeed :> RequirementCheck {
		doc /* A StakeholderNeed states, in the stakeholder's own terms, a capability or quality the enterprise
		 * must provide. Mission requirements are derived from it. */
	}

	abstract requirement stakeholderNeeds : StakeholderNeed[0..*] nonunique :> requirementChecks;

	requirement def MissionRequirement :> RequirementCheck {
		doc /* A MissionRequirement states what the enterprise must achieve, with its measures of
		 * effectiveness, to satisfy the stakeholder needs. It is the top of the derivation chain that ends
		 * in component requirements. */
	}

	abstract requirement missionRequirements : MissionRequirement[0..*] nonunique :> requirementChecks;

	requirement def SystemRequirement :> RequirementCheck {
		doc /* A SystemRequirement states what the system of interest, treated as a black box in its context,
		 * must do or be. It is derived from mission requirements and use cases. */
	}

	abstract requirement systemRequirements : SystemRequirement[0..*] nonunique :> requirementChecks;

	requirement def ComponentRequirement :> RequirementCheck {
		doc /* A ComponentRequirement states what a logical or physical component of the system must do or be,
		 * as derived from the system requirements allocated to it. */
	}

	abstract requirement componentRequirements : ComponentRequirement[0..*] nonunique :> requirementChecks;

	/* System requirements analysis: the black-box system in its context */

	part def SystemOfInterest :> Part {
		doc /* The SystemOfInterest is the system being specified and designed. In the system context it is a
		 * black box whose interfaces, behaviour and requirements are stated without reference to its
		 * internal structure. */
	}

	abstract part systemsOfInterest : SystemOfInterest[0..*] nonunique :> parts;

	part def ExternalSystem :> Part {
		doc /* An ExternalSystem is a system outside the boundary of the system of interest that it exchanges
		 * matter, energy or information with. */
	}

	abstract part externalSystems : ExternalSystem[0..*] nonunique :> parts;

	part def User :> Part {
		doc /* A User is a person, or class of persons, who interacts with the system of interest across its
		 * boundary: an operator, maintainer or other human actor. */
	}

	abstract part users : User[0..*] nonunique :> parts;

	part def Environment :> Part {
		doc /* An Environment is the natural or induced physical environment that surrounds the system of
		 * interest and acts on it: terrain, weather, radiation, orbit, and so on. */
	}

	abstract part environments : Environment[0..*] nonunique :> parts;

	part def SystemContext :> Part {
		doc /* A SystemContext is the composition of the system of interest with everything it interacts with.
		 * It is the subject of system use cases and the frame in which the system's external interfaces
		 * are defined. */

		part systemOfInterest : SystemOfInterest[1] :> systemsOfInterest {
			doc /* The one system of interest this context is drawn around. */
		}

		part externalSystems : ExternalSystem[0..*] :> OOSEM::externalSystems {
			doc /* The external systems the system of interest interacts with. */
		}

		part users : User[0..*] :> OOSEM::users {
			doc /* The users who interact with the system of interest. */
		}

		part environment : Environment[0..1] :> environments {
			doc /* The physical environment the system of interest operates in. */
		}
	}

	abstract part systemContexts : SystemContext[0..*] nonunique :> parts;

	use case def SystemUseCase :> UseCase {
		doc /* A SystemUseCase describes a goal an actor achieves through the system of interest. Its subject
		 * is the system context, so its scenarios show the black-box system exchanging inputs and outputs
		 * with its actors. */
	}

	abstract use case systemUseCases : SystemUseCase[0..*] nonunique :> useCases;

	item def IOEntity :> Item {
		doc /* An IOEntity is something that crosses the boundary of the system of interest: an item of matter,
		 * energy or information it receives or produces. */
	}

	abstract item ioEntities : IOEntity[0..*] nonunique :> items;

	item def Store :> Item {
		doc /* A Store holds items, typically data, that persist between the actions of a scenario so that one
		 * action can deposit what a later one retrieves. */
	}

	abstract item stores : Store[0..*] nonunique :> items;

	/* Logical architecture */

	part def LogicalComponent :> Part {
		doc /* A LogicalComponent is a part of the logical decomposition of the system of interest: it is
		 * defined by the functions and interfaces allocated to it, independent of the technology that will
		 * realise it. */
	}

	abstract part logicalComponents : LogicalComponent[0..*] nonunique :> parts;

	action def LogicalScenario :> Action {
		doc /* A LogicalScenario is a white-box realisation of a system use case scenario: the same externally
		 * visible behaviour, decomposed into actions performed by the logical components and the flows
		 * between them. */
	}

	abstract action logicalScenarios : LogicalScenario[0..*] nonunique :> actions;

	part def Node :> Part {
		doc /* A Node is a locus of the system to which logical components are distributed before they are
		 * realised: a site, a vehicle, a rack. Nodes partition the logical architecture and define the
		 * interfaces between partitions. */
	}

	abstract part nodes : Node[0..*] nonunique :> parts;

	/* Physical architecture */

	part def PhysicalComponent :> Part {
		doc /* A PhysicalComponent realises one or more logical components in a chosen technology. Hardware,
		 * software, data and operational procedures are its kinds. */
	}

	abstract part physicalComponents : PhysicalComponent[0..*] nonunique :> parts;

	part def HardwareComponent :> PhysicalComponent {
		doc /* A HardwareComponent is a physical component realised in hardware. */
	}

	abstract part hardwareComponents : HardwareComponent[0..*] nonunique :> physicalComponents;

	part def SoftwareComponent :> PhysicalComponent {
		doc /* A SoftwareComponent is a physical component realised in software. */
	}

	abstract part softwareComponents : SoftwareComponent[0..*] nonunique :> physicalComponents;

	part def OperationalProcedure :> PhysicalComponent {
		doc /* An OperationalProcedure is a physical component realised as a procedure that a user follows
		 * rather than as hardware or software. */
	}

	abstract part operationalProcedures : OperationalProcedure[0..*] nonunique :> physicalComponents;

	item def DataComponent :> Item {
		doc /* A DataComponent is data the physical architecture stores or transfers, when the data itself is a
		 * design element rather than a value flowing through one. */
	}

	abstract item dataComponents : DataComponent[0..*] nonunique :> items;

	/* Lifecycle */

	enum def EnterpriseStateKind {
		doc /* Whether a model of the enterprise or system describes what exists today or what is proposed. */
		asIs;
		toBe;
	}

	metadata def <asIs> AsIs {
		doc /* Marks an as-is model: the enterprise or system as it exists before the system of interest is
		 * introduced. */
		attribute kind : EnterpriseStateKind = EnterpriseStateKind::asIs;
	}

	metadata def <toBe> ToBe {
		doc /* Marks a to-be model: the enterprise or system as proposed. */
		attribute kind : EnterpriseStateKind = EnterpriseStateKind::toBe;
	}

	/* Package template: OOSEMPackage records which method activity a package holds. */

	enum def OOSEMPackageKind {
		enterpriseModel;
		stakeholderNeeds;
		missionRequirements;
		systemRequirements;
		logicalArchitecture;
		physicalArchitecture;
		analysisModel;
		verificationModel;
		tradeStudies;
	}

	metadata def OOSEMPackage {
		doc /* Classifies a package by the OOSEM activity whose artefacts it holds. */
		:> annotatedElement : SysML::Package;
		attribute kind : OOSEMPackageKind;
		attribute description : String[0..1];
	}

	/* Aliases for the earlier OOSEM stub's MOE/MoP names; prefer #moe and #mop. */

	package 'OOSEM Measures' {
		alias MOE for ParametersOfInterestMetadata::measuresOfEffectiveness;
		alias MoP for ParametersOfInterestMetadata::measuresOfPerformance;
	}

	/* Semantic metadata: the user-defined keywords of the method */

	metadata def <enterprise> EnterpriseMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = enterprises meta SysML::Usage;
	}

	metadata def <stakeholderRole> StakeholderMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = stakeholderParts meta SysML::Usage;
	}

	metadata def <causalAnalysis> CausalAnalysisMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::AnalysisCaseDefinition;
		:> annotatedElement : SysML::AnalysisCaseUsage;
		:>> baseType = causalAnalyses meta SysML::Usage;
	}

	metadata def <enterpriseUseCase> EnterpriseUseCaseMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::UseCaseDefinition;
		:> annotatedElement : SysML::UseCaseUsage;
		:>> baseType = enterpriseUseCases meta SysML::Usage;
	}

	metadata def <stakeholderNeed> StakeholderNeedMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = stakeholderNeeds meta SysML::Usage;
	}

	metadata def <missionRequirement> MissionRequirementMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = missionRequirements meta SysML::Usage;
	}

	metadata def <systemRequirement> SystemRequirementMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = systemRequirements meta SysML::Usage;
	}

	metadata def <componentRequirement> ComponentRequirementMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = componentRequirements meta SysML::Usage;
	}

	metadata def <system> SystemOfInterestMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = systemsOfInterest meta SysML::Usage;
	}

	metadata def <externalSystem> ExternalSystemMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = externalSystems meta SysML::Usage;
	}

	metadata def <user> UserMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = users meta SysML::Usage;
	}

	metadata def <environment> EnvironmentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = environments meta SysML::Usage;
	}

	metadata def <systemContext> SystemContextMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = systemContexts meta SysML::Usage;
	}

	metadata def <systemUseCase> SystemUseCaseMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::UseCaseDefinition;
		:> annotatedElement : SysML::UseCaseUsage;
		:>> baseType = systemUseCases meta SysML::Usage;
	}

	metadata def <ioEntity> IOEntityMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = ioEntities meta SysML::Usage;
	}

	metadata def <store> StoreMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = stores meta SysML::Usage;
	}

	metadata def <logical> LogicalComponentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = logicalComponents meta SysML::Usage;
	}

	metadata def <logicalScenario> LogicalScenarioMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = logicalScenarios meta SysML::Usage;
	}

	metadata def <node> NodeMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = nodes meta SysML::Usage;
	}

	metadata def <physical> PhysicalComponentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = physicalComponents meta SysML::Usage;
	}

	metadata def <hardware> HardwareComponentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = hardwareComponents meta SysML::Usage;
	}

	metadata def <software> SoftwareComponentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = softwareComponents meta SysML::Usage;
	}

	metadata def <operationalProcedure> OperationalProcedureMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = operationalProcedures meta SysML::Usage;
	}

	metadata def <data> DataComponentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = dataComponents meta SysML::Usage;
	}
}
