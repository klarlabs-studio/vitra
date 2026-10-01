package desktop

import "os"

// SetBeforeOpenHook runs fn after a path is authorized and before the file is
// opened, so tests can race the check. It returns a restore function.
func SetBeforeOpenHook(fn func()) (restore func()) {
	prev := beforeOpen
	beforeOpen = fn
	return func() { beforeOpen = prev }
}

// ForceFdPathFallback makes open verify files without looking up their path,
// as on systems that cannot. It returns a restore function.
func ForceFdPathFallback() (restore func()) {
	prev := lookupFdPath
	lookupFdPath = func(*os.File) (string, error) { return "", errFdPathUnsupported }
	return func() { lookupFdPath = prev }
}

// SetBeforeLaunchHook runs fn after path.open has verified the opened file
// and before it hands the path to the opener. It returns a restore function.
func SetBeforeLaunchHook(fn func()) (restore func()) {
	prev := beforeLaunch
	beforeLaunch = fn
	return func() { beforeLaunch = prev }
}
