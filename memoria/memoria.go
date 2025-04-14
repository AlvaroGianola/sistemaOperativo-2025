package main

import (
	"fmt"
	"net/http"

	memoriaUtils "github.com/sisoputnfrba/tp-golang/memoria/memoriaUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
)

func main() {

	// Inicializa el logger que usará todo el módulo Memoria
	clientUtils.ConfigurarLogger("memoria.log")

	// Carga la configuración desde el archivo config.json
	globalsMemoria.MemoriaConfig = memoriaUtils.IniciarConfiguracion("config.json")

	// Crea el multiplexer HTTP y registra los endpoints que usará Memoria
	mux := http.NewServeMux()

	// Endpoints que reciben peticiones desde CPU y Kernel
	mux.HandleFunc("/cpu", memoriaUtils.RecibirPeticionCpu)
	mux.HandleFunc("/kernel", memoriaUtils.RecibirPeticionKernel)

	// Levanta el servidor en el puerto definido por configuración
	direccion := fmt.Sprintf(":%d", globalsMemoria.MemoriaConfig.PortMemory)
	fmt.Printf("[Memoria] Servidor escuchando en puerto %d...\n", globalsMemoria.MemoriaConfig.PortMemory)

	err := http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}
}
