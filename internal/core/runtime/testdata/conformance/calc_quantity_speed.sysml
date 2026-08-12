// A calculation whose result carries its unit: 10 [m] / 4 [s] is 2.5 [m/s].
// Its trace names the unit at every step it appears in, since a magnitude
// without its unit says nothing about what was computed.
package test {
	public import SI::*;
	public import ISQ::*;

	calc def Speed {
		return : ISQSpaceTime::SpeedValue = 10.0 [m] / 4.0 [s];
	}
}
