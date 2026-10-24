//go:build !linux

package receiver

const platformSupported = false

func defaultRuntime() runtime {
	return runtime{}
}
