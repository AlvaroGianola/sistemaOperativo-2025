package main

import (
	"fmt"
	"net/http"

	cpuUtils "github.com/sisoputnfrba/tp-golang/cpu/cpuUtils"
	globalscpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	// Inicializa el logger para registrar los eventos del módulo CPU
	clientUtils.ConfigurarLogger("cpu.log")

	// Carga la configuración desde el archivo config.json
	globalscpu.CpuConfig = cpuUtils.IniciarConfiguracion("config.json")

	// Crea un enrutador HTTP (mux) y registra los endpoints que atenderá la CPU
	mux := http.NewServeMux()
	mux.HandleFunc("/recibirProceso", cpuUtils.RecibirProceso)
	mux.HandleFunc("/recibirInterrupcion", cpuUtils.RecibirInterrupcion)

	// Envía el handshake al Kernel con los datos de IP y puerto de esta CPU
	cpuUtils.EnviarHandshakeAKernel()

	// Obtiene el puerto configurado para levantar el servidor
	direccion := fmt.Sprintf(":%d", globalscpu.CpuConfig.PortCpu)
	fmt.Printf("[CPU] Servidor escuchando en puerto %d...\n", globalscpu.CpuConfig.PortCpu)

	err := http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}

}
