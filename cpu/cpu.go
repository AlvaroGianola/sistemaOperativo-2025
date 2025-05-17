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

	// Carga la configuración desde el archivo config.json
	globalscpu.CpuConfig = cpuUtils.IniciarConfiguracion("config.json")

	args := os.Args

	indentificador := args[1]
	if len(args) < 2 {
		fmt.Println("Error: se debe pasar el identificador de la CPU como argumento")
		os.Exit(1)
	}

	globalscpu.SetIdentificador(indentificador)

	// Inicializa el logger para registrar los eventos del módulo CPU
	clientUtils.ConfigurarLogger("cpu" + indentificador + ".log")

	//-----------------------------------
	//Inicializo proceso a modo de prueba
	proceso := cpuUtils.Proceso{
		Pid: 1,
		Pc:  0,
	}

	cpuUtils.HandleProceso(proceso)
	//------------------------------------

	// Crea un enrutador HTTP (mux) y registra los endpoints que atenderá la CPU
	mux := http.NewServeMux()
	mux.HandleFunc("/dispatch", cpuUtils.RecibirProceso)
	mux.HandleFunc("/recibirInterrupcion", cpuUtils.RecibirInterrupcion)

	puertoLibre, err := clientUtils.EncontrarPuertoDisponible(globalscpu.CpuConfig.IpCpu, globalscpu.CpuConfig.PortCpu)
	if err != nil {
		panic(err)
	}

	// Envía el handshake al Kernel con los datos de IP y puerto de esta CPU
	cpuUtils.EnviarHandshakeAKernel(indentificador, puertoLibre)

	// Obtiene el puerto configurado para levantar el servidor
	direccion := fmt.Sprintf("%s:%d", globalscpu.CpuConfig.IpCpu, puertoLibre)
	fmt.Printf("[CPU] Servidor escuchando en puerto %d...\n", puertoLibre)

	err = http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}

}
