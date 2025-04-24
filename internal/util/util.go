package util

import (
	"runtime"
)

func SetMaxProcs() {
	// Allow as many threads as we have cores unless the user specified a value.
	numProcs := runtime.NumCPU()
	runtime.GOMAXPROCS(numProcs)
	// Check if the setting was successful.
	actualNumProcs := runtime.GOMAXPROCS(0)
	if actualNumProcs != numProcs {
		// log.Info().Msgf("Specified max procs of %d but using %d", numProcs, actualNumProcs)
	}
}
