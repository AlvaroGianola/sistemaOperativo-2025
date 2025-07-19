package main

import (
	"fmt"
	"net/http"
	"os"

	cpuUtils "github.com/sisoputnfrba/tp-golang/cpu/cpuUtils"
	globalscpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	// Validar argumentos
	if len(os.Args) < 2 {
		fmt.Println("Error: se debe pasar el identificador de la CPU como argumento")
		os.Exit(1)
	}
	identificador := os.Args[1]

	// Configurar logger
	clientUtils.ConfigurarLogger("cpu" + identificador + ".log")

	// Configurar CPU
	globalscpu.CpuConfig = cpuUtils.IniciarConfiguracion("config.json")
	globalscpu.SetIdentificador(identificador)
	cpuUtils.ObtenerInfoMemoria()

	// Registrar endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/recibirProceso", cpuUtils.RecibirProceso)
	mux.HandleFunc("/recibirInterrupcion", cpuUtils.RecibirInterrupcion)

	// Buscar puerto disponible y levantar servidor
	listener, puertoLibre, err := clientUtils.EncontrarPuertoDisponible(globalscpu.CpuConfig.IpCpu, globalscpu.CpuConfig.PortCpu)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[CPU %s] Escuchando en puerto %d...\n", identificador, puertoLibre)

	// Hacer handshake al Kernel
	cpuUtils.EnviarHandshakeAKernel(identificador, puertoLibre)

	//TODO: HANDSHAKE CON MEMORIA (CAMBIANDO PUERTO)

	// Servir usando el listener ya abierto
	err = http.Serve(listener, mux)
	if err != nil {
		panic(err)
	}
}
