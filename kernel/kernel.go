package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	globalsKernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	kernelUtils "github.com/sisoputnfrba/tp-golang/kernel/kernelUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	// Inicializa el logger que usará todo el módulo Kernel
	clientUtils.ConfigurarLogger("kernel.log")

	// Carga la configuración desde el archivo config.json
	globalsKernel.KernelConfig = kernelUtils.IniciarConfiguracion("config.json")

	kernelUtils.Plp = kernelUtils.InciarPlp()
	kernelUtils.Pmp = kernelUtils.IniciarPmp()

	args := os.Args
	filePath := args[1]
	tamProc, err := strconv.Atoi(args[2])
	if err != nil {
		panic(err)
	}

	// Crea el multiplexer HTTP para registrar handlers
	mux := http.NewServeMux()

	// Endpoints para handshakes y resultados:
	// CPUs envían handshake a /cpus
	mux.HandleFunc("/cpus", kernelUtils.RegistrarCpu)

	// Las CPUs envían resultados o finalización a /resultadoProcesos
	mux.HandleFunc("/resultadoProcesos", kernelUtils.ResultadoProcesos)

	// IOs envían handshake a /ios
	mux.HandleFunc("/ios", kernelUtils.RegistrarIo)

	// IOs envian resultados a /resultadoIos
	mux.HandleFunc("/finIos", kernelUtils.FinIos)

	mux.HandleFunc("/desconexionIos", kernelUtils.DesconexionIos)

	// Levanta el servidor en el puerto definido en el archivo de configuración
	direccion := fmt.Sprintf("%s:%d", globalsKernel.KernelConfig.IpKernel, globalsKernel.KernelConfig.PortKernel)
	fmt.Printf("[Kernel] Servidor HTTP escuchando en puerto %d...\n", globalsKernel.KernelConfig.PortKernel)

	go kernelUtils.EsperarEnter()
	go kernelUtils.IniciarKernel(filePath, uint(tamProc))

	err = http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}
}
