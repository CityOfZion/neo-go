/*
Package binary provides binary serialization routines.
*/
package binary

// Serialize serializes any given item into a byte slice. It works for all
// regular VM types (not ones from interop package) and allows to save them in
// storage or pass into Notify and then Deserialize them on the next run or in
// the external event receiver. It uses `System.Binary.Serialize` syscall.
func Serialize(item interface{}) []byte {
	return nil
}

// Deserialize unpacks previously serialized value from a byte slice, it's the
// opposite of Serialize. It uses `System.Binary.Deserialize` syscall.
func Deserialize(b []byte) interface{} {
	return nil
}

// Base64Encode encodes given byte slice into a base64 string and returns byte
// representation of this string. It uses `System.Binary.Base64Encode` interop.
func Base64Encode(b []byte) []byte {
	return nil
}

// Base64Decode decodes given base64 string represented as a byte slice into
// byte slice. It uses `System.Binary.Base64Decode` interop.
func Base64Decode(b []byte) []byte {
	return nil
}

// Base58Encode encodes given byte slice into a base58 string and returns byte
// representation of this string. It uses `System.Binary.Base58Encode` syscall.
func Base58Encode(b []byte) []byte {
	return nil
}

// Base58Decode decodes given base58 string represented as a byte slice into
// a new byte slice. It uses `System.Binary.Base58Decode` syscall.
func Base58Decode(b []byte) []byte {
	return nil
}
