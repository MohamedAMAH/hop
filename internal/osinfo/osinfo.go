/* Package osinfo describes the two path-separator families hop rewrites between. */
package osinfo

import "runtime"

/* OS identifies a path-separator family, not a specific operating system. */
type OS int

const (
	Unix    OS = iota // Unix uses forward-slash path separators.
	Windows           // Windows uses backslash path separators.
)

/* Current reports the OS family of the machine hop is running on. */
func Current() OS {
	if runtime.GOOS == "windows" {
		return Windows
	}
	return Unix
}

/* Sep returns the path separator for this OS family. */
func (o OS) Sep() string {
	if o == Windows {
		return `\`
	}
	return "/"
}
