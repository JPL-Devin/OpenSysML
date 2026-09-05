// A binary connector names itself only ahead of `from`; without `from` the
// first token is the first end (KerML.xtext:836 BinaryConnectorDeclaration),
// which may carry a crossing multiplicity (:855) or a name referencing its
// feature (:856).
package BinaryConnector {
	class Engine;
	class Tank { feature main : Engine; }

	class Vehicle {
		feature eng : Engine;
		feature tanks : Tank;

		connector eng to tanks.main;
		connector [0..1] eng to [1..*] tanks.main;
		connector e ::> eng to tanks.main;
		connector e references eng to tanks.main;
		connector link from eng to tanks.main;
		connector binary : Links::BinaryLink from eng to tanks.main;
	}
}
