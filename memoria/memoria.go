package main

import (
	"fmt"
	"net/http"

	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	memoriaUtils "github.com/sisoputnfrba/tp-golang/memoria/memoriaUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {

	// Inicializa el logger que usará todo el módulo Memoria
	clientUtils.ConfigurarLogger("memoria.log")

	// Carga la configuración desde el archivo config.json
	globalsMemoria.MemoriaConfig = memoriaUtils.IniciarConfiguracion("config.json")

	// Crea el multiplexer HTTP y registra los endpoints que usará Memoria
	mux := http.NewServeMux()

	// Endpoints que reciben peticiones desde Kernel
	mux.HandleFunc("/iniciarProceso", memoriaUtils.IniciarProceso)
	mux.HandleFunc("/finalizarProceso", memoriaUtils.FinalizarProceso)
	mux.HandleFunc("/suspenderProceso", memoriaUtils.SuspenderProceso)
	mux.HandleFunc("/desuspenderProceso", memoriaUtils.DesuspenderProceso)

	// Endpoints que reciben peticiones desde CPU
	mux.HandleFunc("/obtenerConfiguracionMemoria", memoriaUtils.ObtenerConfiguracionMemoria)
	mux.HandleFunc("/siguienteInstruccion", memoriaUtils.SiguienteInstruccion)
	mux.HandleFunc("/accederMarcoUsuario", memoriaUtils.AccederMarcoUsuario)
	mux.HandleFunc("/readPagina", memoriaUtils.LeerPagina)
	mux.HandleFunc("/writePagina", memoriaUtils.EscribirPagina)
	mux.HandleFunc("/writeMemoria", memoriaUtils.EscribirDireccionFisica)
	mux.HandleFunc("/readMemoria", memoriaUtils.LeerDireccionFisica)

	// Levanta el servidor en el puerto definido por configuración
	direccion := fmt.Sprintf("%s:%d", globalsMemoria.MemoriaConfig.IpMemory, globalsMemoria.MemoriaConfig.PortMemory)
	fmt.Printf("[Memoria] Servidor escuchando en puerto %d...\n", globalsMemoria.MemoriaConfig.PortMemory)

	err := http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}
}
