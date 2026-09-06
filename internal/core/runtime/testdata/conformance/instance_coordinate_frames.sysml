// A coordinate frame is a value: a usage typed CoordinateFrame with its `mRefs`
// evaluates to the frame it declares, `CoordinateFrame/` divides every axis by a
// unit (Annex A's `velocityCF = spatialCF/s`), a vector literal over a frame is a
// vector quantity whose `mRef` is that frame, and `transform` re-expresses such a
// vector in the target frame of a placement, a translation–rotation sequence, an
// affine matrix or the null transformation. A placement states where the target
// sits in the source (`v_source = origin + B v_target`), so transform applies the
// inverse and an inverse sequence round-trips. A rotation's angle is read in its
// unit: the library's degree is `1.745329E-02 rad`, exact to seven digits, so a
// quarter turn in degrees agrees with one in radians to that precision. A
// measurement scale is a one-dimensional frame: Celsius converts to and from
// kelvin through its placement on K, and UTC is a value whose conversion the
// library gives no mapping for.
package test {
	public import ISQ::*;
	public import SI::*;
	public import ISQSpaceTime::*;
	public import MeasurementReferences::*;
	public import QuantityCalculations::*;
	public import VectorCalculations::*;
	private import ScalarValues::*;

	part def Rig {
		attribute spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }
		attribute velocityCF : CartesianVelocity3dCoordinateFrame[1] = spatialCF / s;
		attribute accelerationCF : CartesianAcceleration3dCoordinateFrame[1] = velocityCF / s;
		attribute velocityAxes : SpeedUnit[3] = velocityCF.mRefs;
		attribute scaledCF : CoordinateFrame = MeasurementRefCalculations::'CoordinateFrame*'(spatialCF, s);
		attribute sameCF : Boolean = spatialCF / s == velocityCF;

		attribute p : Position3dVector = (1.0, 2.0, 3.0) [spatialCF];
		attribute pFrame : CartesianSpatial3dCoordinateFrame = p.mRef;
		attribute pNums : Real[3] = p.num;
		attribute pIsSpatial : Boolean = p.mRef == spatialCF;
		attribute uniform : Quantities::VectorQuantityValue = VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m];
		attribute uniformUnit : LengthUnit = uniform.mRef;

		attribute datum : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (mm, mm, mm); }
		attribute metres : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }
		attribute shifted : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : CoordinateFramePlacement {
				:>> source = datum;
				:>> origin = (10.0, 20.0, 30.0) [datum];
				:>> basisDirections = ((0.0, 2.0, 0.0) [datum], (-1.0, 0.0, 0.0) [datum], (0.0, 0.0, 1.0) [datum]);
			}
		}
		attribute placed : Position3dVector = transform(shifted.transformation, (11.0, 22.0, 33.0) [datum]);
		attribute placedFrame : Boolean = placed.mRef == shifted;
		attribute wrongSource : Position3dVector = transform(shifted.transformation, (0.011, 0.022, 0.033) [metres]);

		attribute turned : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : TranslationRotationSequence {
				:>> source = datum;
				:>> elements = (new Translation((100.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 1.5707963267948966 [rad]));
			}
		}
		attribute back : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : TranslationRotationSequence {
				:>> source = turned;
				:>> elements = (new Rotation((0.0, 0.0, 1.0) [turned], -1.5707963267948966 [rad]), new Translation((0.0, 100.0, 0.0) [turned]));
			}
		}
		attribute q : Position3dVector = (101.0, 1.0, 5.0) [datum];
		attribute qTurned : Position3dVector = transform(turned.transformation, q);
		attribute qBack : Position3dVector = transform(back.transformation, qTurned);
		attribute qBackFrame : Boolean = qBack.mRef == back;
		attribute trs : TranslationRotationSequence {
			:>> source = datum;
			:>> target = turned;
			:>> elements = (new Translation((100.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 1.5707963267948966 [rad]));
		}
		attribute sameTransformation : Boolean = turned.transformation == trs;
		attribute otherTransformation : Boolean = back.transformation == trs;
		attribute degrees : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : TranslationRotationSequence {
				:>> source = datum;
				:>> elements = (new Translation((100.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 90 ['°']));
			}
		}
		attribute qDegrees : Position3dVector = transform(degrees.transformation, q);
		attribute degreesAgree : Boolean = RealFunctions::abs(qDegrees.num#(1) - qTurned.num#(1)) < 1.0e-6
			and RealFunctions::abs(qDegrees.num#(2) - qTurned.num#(2)) < 1.0e-6;
		attribute extrinsic : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : TranslationRotationSequence {
				:>> source = datum;
				:>> elements = (new Rotation((0.0, 0.0, 1.0) [datum], 1.5707963267948966 [rad]), new Rotation((1.0, 0.0, 0.0) [datum], 1.5707963267948966 [rad], false));
			}
		}
		attribute qExtrinsic : Position3dVector = transform(extrinsic.transformation, (1.0, 2.0, 3.0) [datum]);
		attribute intrinsic : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : TranslationRotationSequence {
				:>> source = datum;
				:>> elements = (new Rotation((0.0, 0.0, 1.0) [datum], 1.5707963267948966 [rad]), new Rotation((1.0, 0.0, 0.0) [datum], 1.5707963267948966 [rad]));
			}
		}
		attribute qIntrinsic : Position3dVector = transform(intrinsic.transformation, (1.0, 2.0, 3.0) [datum]);

		attribute affine : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : AffineTransformationMatrix3d {
				:>> source = datum;
				:>> rotationMatrix { :>> elements = (0.0, -1.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0); }
				:>> translationVector { :>> elements = (100.0, 0.0, 0.0); }
			}
		}
		attribute qAffine : Position3dVector = transform(affine.transformation, q);
		attribute unmoved : CartesianSpatial3dCoordinateFrame[1] {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : NullTransformation { :>> source = datum; }
		}
		attribute qNull : Position3dVector = transform(unmoved.transformation, q);
		attribute qNullFrame : Boolean = qNull.mRef == unmoved;

		attribute celsius : IntervalScale = SI::'°C_abs';
		attribute celsiusUnit : TemperatureUnit = celsius.unit;
		attribute warm : ThermodynamicTemperatureValue = ConvertQuantity(300.0 [K], SI::'°C_abs');
		attribute warmBack : ThermodynamicTemperatureValue = ConvertQuantity(26.85 [SI::'°C_abs'], K);
		attribute freezing : ThermodynamicTemperatureValue = ConvertQuantity(0.0 [SI::'°C_abs'], K);
		attribute onScale : ThermodynamicTemperatureValue = 21.5 [SI::'°C_abs'];
		attribute onScaleRef = onScale.mRef;
		attribute onScaleIsCelsius : Boolean = onScale.mRef == celsius;
		attribute utc : Time::TimeScale = Time::UTC;
		attribute utcUnit : DurationUnit = Time::UTC.unit;
		attribute epoch : Time::TimeInstantValue = 0 [Time::UTC];
		attribute utcToSeconds = ConvertQuantity(3 [Time::UTC], s);
		attribute secondsToUtc = ConvertQuantity(3 [s], Time::UTC);
		attribute utcLabel : String = MeasurementRefCalculations::ToString(Time::UTC);
	}
}
