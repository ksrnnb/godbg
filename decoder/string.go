/**
 * this package is from delve v0.7.0-alpha
 * https://github.com/go-delve/delve/blob/v0.7.0-alpha/dwarf/util/util.go
 */
package decoder

import "bytes"

func ParseString(data *bytes.Buffer) (string, uint32) {
	str, err := data.ReadString(0x0)
	if err != nil {
		panic("Could not parse string")
	}

	return str[:len(str)-1], uint32(len(str))
}
