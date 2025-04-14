package main

import (
	"log"
	"net/http"

	"github.com/sisoputnfrba/tp-golang/kernel/kernelUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	clientUtils.ConfigurarLogger("kernel.log")
	kernelUtils.KernelConfig = kernelUtils.IniciarConfiguracion("config.json")

	mux := http.NewServeMux()

	mux.HandleFunc("/cpus", kernelUtils.RegistrarCpu)
	mux.HandleFunc("/resultadoProcesos", kernelUtils.ResultadoProcesos)
	mux.HandleFunc("/ios", kernelUtils.RegistrarIo)

	log.Printf("Kernel escuchando en 8083")

	err := http.ListenAndServe(":8083", mux)
	if err != nil {
		panic(err)
	}
}
