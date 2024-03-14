package util

// Unpacks a slice into arguments
// If the slice has less elements than variables passed in, the rest of the variables are not modified
// If the slice has more elements than the variables passed in, the additional elements are ignored
// Copied and adjusted from https://stackoverflow.com/a/19832661
func Unpack[T any](toUnpack []T, unpackInto ...*T) {
	if len(toUnpack) > len(unpackInto) {
		for i := range unpackInto {
			*unpackInto[i] = toUnpack[i]
		}
	} else {
		for i, str := range toUnpack {
			*unpackInto[i] = str
		}
	}
}
