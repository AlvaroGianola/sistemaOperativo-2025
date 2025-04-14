package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/cpu/cpuUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	clientUtils.ConfigurarLogger("cpu.log")

	cpuUtils.CpuConfig = cpuUtils.IniciarConfiguracion("config.json")

	mux := http.NewServeMux()

	mux.HandleFunc("/recibirProceso", cpuUtils.RecibirProceso)
	mux.HandleFunc("/recibirInterrupcion", cpuUtils.RecibirInterrupcion)

	cpuUtils.EnviarHandshakeAKernel()

	err := http.ListenAndServe(":8081", mux)
	if err != nil {
		panic(err)
	}

}
