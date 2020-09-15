package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	defaultConfig = "config.yaml"
	exampleConfig = "config-example.yaml"
	myName        = `
✄╔════╗
✄╚══╗═║
✄──╔╝╔╝╔══╗╔╗╔╗╔══╗╔═╗╔══╗
✄─╔╝╔╝─║║═╣║║║║║══╣║╔╝║╔╗║
✄╔╝═╚═╗║║═╣║╚╝║╠══║║║─║╚╝║
✄╚════╝╚══╝╚══╝╚══╝╚╝─╚══╝
`
	LINE = "----------------------------------------"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func main() {
	fmt.Println(LINE)
	fmt.Print("Power by")
	log.Info().Msgf("%s", myName)
	// fmt.Println(myName)
	fmt.Println(LINE)
	setMaxProcs()

}

func setMaxProcs() {
	// Allow as many threads as we have cores unless the user specified a value.
	numProcs := runtime.NumCPU()
	runtime.GOMAXPROCS(numProcs)
	// Check if the setting was successful.
	actualNumProcs := runtime.GOMAXPROCS(0)
	if actualNumProcs != numProcs {
		log.Info().Msgf("Specified max procs of %d but using %d", numProcs, actualNumProcs)
	}
}
