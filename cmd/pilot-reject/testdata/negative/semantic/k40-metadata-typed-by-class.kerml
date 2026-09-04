// Invalid: a metadata feature must be typed by a metaclass, not an ordinary class (KerML 8.3.4.11 validateMetadataFeatureMetadata; pilot validateMetadataFeatureMetadata).
package K40MetadataTypedByClass {
    class M;
    class C;
    @M about C;
}
