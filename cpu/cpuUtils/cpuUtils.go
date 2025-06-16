package cpuUtils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	cacheUtils "github.com/sisoputnfrba/tp-golang/cpu/cache"
	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	mmuUtils "github.com/sisoputnfrba/tp-golang/cpu/mmu"
	tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

const (
	// Constantes para los códigos de operación
	NOOP        = "NOOP"
	WRITE       = "WRITE"
	READ        = "READ"
	GOTO        = "GOTO"
	IO          = "IO"
	INIT_PROC   = "INIT_PROC"
	DUMP_MEMORY = "DUMP_MEMORY"
	EXIT        = "EXIT"
	// Constantes para los tipos de interrupción
	INVALID = "INVALID"
)

// Representa un proceso con su PID y su Program Counter (PC)
type Proceso struct {
	Pid int `json:"pid"`
	Pc  int `json:"pc"`
}

// Inicializa la configuración leyendo el archivo json indicado

func IniciarConfiguracion(filePath string) *globalsCpu.Config {
	config := &globalsCpu.Config{} // Aca creamos el contenedor donde irá el JSON

	configFile, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(config)
	if err != nil {
		panic("Error al decodificar config: " + err.Error())
	}

	return config
}

func ObtenerInfoMemoria() {
	respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"obtenerConfiguracionMemoria",
		clientUtils.Paquete{},
	)

	if respuesta == nil {
		clientUtils.Logger.Error("No se pudo obtener la configuración de Memoria")
		panic("Memoria no disponible")
	}

	tamPagina, _ := strconv.Atoi(string(respuesta[0]))
	niveles, _ := strconv.Atoi(string(respuesta[1]))
	entradas, _ := strconv.Atoi(string(respuesta[2]))

	globalsCpu.Memoria.TamanioPagina = tamPagina
	globalsCpu.Memoria.NivelesPaginacion = niveles
	globalsCpu.Memoria.CantidadEntradas = entradas

	clientUtils.Logger.Info("Informacion de memoria obtenida correctamente",
		"Tamaño página", tamPagina,
		"Niveles", niveles,
		"Entradas por tabla", entradas,
	)
}

// Recibe un proceso del Kernel y lo loguea
func RecibirProceso(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)

	spid := paquete.Valores[0]
	spc := paquete.Valores[1]

	pid, err := strconv.Atoi(spid)
	if err != nil {
		clientUtils.Logger.Error("Error al convertir PID a int")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pc, err := strconv.Atoi(spc)
	if err != nil {
		clientUtils.Logger.Error("Error al convertir PC a int")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	clientUtils.Logger.Info(fmt.Sprintf("## Llega proceso - PID: %d, PC: %d", pid, pc))
	globalsCpu.ProcesoActual.Pc = pc
	globalsCpu.ProcesoActual.Pid = pid

	HandleProceso(globalsCpu.ProcesoActual)

	w.WriteHeader(http.StatusOK)
}

func PedirSiguienteInstruccionMemoria() (string, bool) {

	valores := []string{strconv.Itoa(globalsCpu.ProcesoActual.Pid), strconv.Itoa(globalsCpu.ProcesoActual.Pc)}
	paquete := clientUtils.Paquete{Valores: valores}
	instruccion := clientUtils.EnviarPaqueteConRespuestaBody(globalsCpu.CpuConfig.IpMemory, globalsCpu.CpuConfig.PortMemory, "siguienteInstruccion", paquete)

	if instruccion == nil {
		clientUtils.Logger.Error("No se recibió respuesta de Memoria")
		return "", false
	}
	return string(instruccion), true
}

// Envia handshake al Kernel con IP y puerto de esta CPU
func EnviarHandshakeAKernel(indentificador string, puertoLibre int) {

	puertoCpu := strconv.Itoa(puertoLibre)

	valores := []string{indentificador, globalsCpu.CpuConfig.IpCpu, puertoCpu}

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "cpus") //IP y Puerto de la CPU

}

func EnviarHandshakeAMemoria(identificador string, puertoLibre int) {
	puertoCpu := strconv.Itoa(puertoLibre)

	valores := []string{identificador, globalsCpu.CpuConfig.IpCpu, puertoCpu}

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpMemory, globalsCpu.CpuConfig.PortMemory, "cpus") //IP y Puerto de la CPU
}

// handleProceso será el núcleo del ciclo de instrucción en Checkpoint 2 en adelante
// Por ahora queda como placeholder para mantener la estructura modular
func HandleProceso(proceso *globalsCpu.Proceso) {

	for {

		//#FETCH
		instruccion, ok := PedirSiguienteInstruccionMemoria()
		if !ok {

			clientUtils.Logger.Error("Error al pedir la siguiente instruccion a memoria")
			break
		}
		clientUtils.Logger.Info(fmt.Sprintf("## Instrucción: %s", instruccion))
		//#DECODE
		cod_op, variables := DecodeInstruccion(instruccion)
		clientUtils.Logger.Info(fmt.Sprintf("## Instrucción decodificada: %s, con las variables %s", cod_op, variables))
		//#EXECUTE
		clientUtils.Logger.Info("## Ejecutando instrucción")
		ExecuteInstruccion(proceso, cod_op, variables)
		//#CHECK
		// le pregunto a kernel si hay una interrupción
		hayInterrupcion, ok := PreguntarSiHayInterrupcion()
		if !ok {
			clientUtils.Logger.Error("Error al preguntar si hay interrupción")
		} else {
			if hayInterrupcion == "FALSE" {
				clientUtils.Logger.Info("## No hay interrupción, continuando ejecución")
			} else if hayInterrupcion == "TRUE" {
				clientUtils.Logger.Info("## Hay interrupción, deteniendo ejecución")
				break
			}
		}

		// Si la instrucción es EXIT o INVALIDA, salimos del ciclo
		if cod_op == EXIT {
			LimpiarProceso(globalsCpu.ProcesoActual.Pid)
			clientUtils.Logger.Info("## Proceso finalizado")
			break
		} else if cod_op == IO || cod_op == INIT_PROC || cod_op == DUMP_MEMORY {
			LimpiarProceso(globalsCpu.ProcesoActual.Pid)
			break
		}
		if cod_op == INVALID {
			LimpiarProceso(globalsCpu.ProcesoActual.Pid)
			clientUtils.Logger.Error("## Instrucción inválida, abortando ejecución")
			break
		}
	}

}

func PreguntarSiHayInterrupcion() (string, bool) {
	valores := []string{strconv.Itoa(globalsCpu.ProcesoActual.Pid), strconv.Itoa(globalsCpu.ProcesoActual.Pc)}
	paquete := clientUtils.Paquete{Valores: valores}
	interrupcion := clientUtils.EnviarPaqueteConRespuestaBody(globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "recibirInterrupcion", paquete)

	if interrupcion == nil {
		clientUtils.Logger.Error("No se recibió respuesta de Kernel")
		return "", false
	}
	return string(interrupcion), true
}

// Simula la recepción de una interrupción
func RecibirInterrupcion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("## Llega interrupción al puerto Interrupt")
	w.WriteHeader(http.StatusOK)
}

//----------------------------------------------------------------------

func EnviarResultadoAKernel(pc int, cod_op string, args []string) {
	pcStr := strconv.Itoa(pc)

	ids := []string{globalsCpu.Identificador, pcStr, cod_op}

	valores := append(ids, args...)

	resultadoStr := strings.Join(valores, " ")
	clientUtils.Logger.Info(fmt.Sprintf("valores a enviar a kernel: %s", resultadoStr))

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "resultadoProcesos")
}

func Decode(instruccion string) (op string, args []string) {
	parts := strings.Fields(instruccion)
	if len(parts) == 0 {
		return "", []string{}
	}
	return parts[0], parts[1:]
}

func DecodeInstruccion(instruccion string) (cod_op string, variables []string) {
	cod_op, variables = Decode(instruccion)

	switch cod_op {
	case NOOP, EXIT, DUMP_MEMORY:
		if len(variables) != 0 {
			clientUtils.Logger.Error(fmt.Sprintf("cantidad de parametros recibidos en la instruccion %s incorrecto, no se deben ingresar parametros para esta instruccion", cod_op))
		}
	case GOTO:
		cod_op = GOTO
		if len(variables) != 1 {
			clientUtils.Logger.Error("cantidad de parametros recibidos en la instruccion GOTO incorrecto, se debe ingresar 1 parametro")
		}
	case READ, WRITE, IO, INIT_PROC:
		if len(variables) != 2 {
			clientUtils.Logger.Error(fmt.Sprintf("cantidad de parametros recibidos en la instruccion %s incorrecto, se deben ingresar 2 parametros", cod_op))
		}
	default:
		clientUtils.Logger.Error("Instrucción inválida")
		cod_op = INVALID
	}
	return cod_op, variables
}

func ExecuteInstruccion(proceso *globalsCpu.Proceso, cod_op string, variables []string) {
	switch cod_op {
	case NOOP:
		clientUtils.Logger.Info("## Ejecutando NOOP")
		time.Sleep(2 * time.Second)
		globalsCpu.ProcesoActual.Pc++

	case WRITE:
		clientUtils.Logger.Info("## Ejecutando WRITE")
		direccion := variables[0]
		dato := variables[1]
		direccionInt, err := strconv.Atoi(direccion)

		if err != nil {
			clientUtils.Logger.Error("WRITE: argumento inválido, no es un número")
			return
		}

		writeMemoria(proceso.Pid, direccionInt, dato)
		globalsCpu.ProcesoActual.Pc++
	case READ:
		clientUtils.Logger.Info("## Ejecutando READ")
		direccion := variables[0]
		tamanio := variables[1]
		direccionInt, err := strconv.Atoi(direccion)
		if err != nil {
			clientUtils.Logger.Error("READ: argumento inválido, no es un número")
			return
		}
		tamanioInt, err := strconv.Atoi(tamanio)

		if err != nil {
			clientUtils.Logger.Error("READ: argumento inválido, no es un número")
			return
		}

		readMemoria(proceso.Pid, direccionInt, tamanioInt)
		globalsCpu.ProcesoActual.Pc++
	case GOTO:
		clientUtils.Logger.Info("## Ejecutando GOTO")
		nuevoPC, err := strconv.Atoi(variables[0])
		if err != nil {
			clientUtils.Logger.Warn("GOTO: argumento inválido, no es un número")
			break
		}
		globalsCpu.ProcesoActual.Pc = nuevoPC

	default:
		if cod_op != IO && cod_op != INIT_PROC && cod_op != DUMP_MEMORY && cod_op != EXIT {
			clientUtils.Logger.Error("## Instruccion no reconocida")
		} else {
			Syscall(proceso, cod_op, variables)
		}
	}

}

func Syscall(proceso *globalsCpu.Proceso, cod_op string, variables []string) {
	switch cod_op {
	case IO:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar IO")
		globalsCpu.ProcesoActual.Pc++
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	case INIT_PROC:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar INIT_PROC")
		globalsCpu.ProcesoActual.Pc++
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	case DUMP_MEMORY:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar DUMP_MEMORY")
		globalsCpu.ProcesoActual.Pc++
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	case EXIT:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar EXIT")
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	default:
		clientUtils.Logger.Error("Error, instruccion no reconocida")
	}
}

// Escribir y Leer memoria

func readMemoria(pid int, direccionLogica int, tamanio int) {
	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)

	if( globalsCpu.CpuConfig.CacheEntries > 0) {
		contenido, encontroContenido := cacheUtils.BuscarEnCache(pid, pagina)
		if encontroContenido {
			clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d, Contenido: %s → Cache HIT", pid, pagina, contenido[:tamanio]))
			fmt.Println(contenido[:tamanio])
			return
		}else{
			clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d → Cache MISS", pid, pagina))
		}
	}
		marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
		if err != nil {
			clientUtils.Logger.Error(fmt.Sprintf("READ - Error al obtener marco: %s", err))
			return
		}

		// Log
		clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Dir. lógica: %d → Dir. física: %d", pid, direccionLogica, marco))

		contenido,err := consultaRead(pid, marco, direccionLogica)

		if err != nil {
			clientUtils.Logger.Error(fmt.Sprintf("READ - Error al consultar memoria: %s", err))
			return
		}

		if globalsCpu.CpuConfig.CacheEntries > 0 {
			cacheUtils.AgregarACache(pid, pagina, contenido)
			clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d → Agregando a caché", pid, pagina))
		}

		clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d, Contenido: %s", pid, pagina, contenido[:tamanio]))
		fmt.Println(contenido[:tamanio])

}

func writeMemoria(pid int, direccionLogica int, dato string) {
	// Traducir dirección lógica a física
	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)

	if globalsCpu.CpuConfig.CacheEntries > 0 {
		dato,encontroDato := cacheUtils.BuscarEnCache(pid, pagina)
		if encontroDato {
			clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Página %d, Contenido %s → Cache HIT", pid, pagina, dato))
			err := cacheUtils.ModificarContenidoCache(pid, pagina, dato)
			if err != nil {
				clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error al modificar contenido en cache: %s", err))
				return
			}
		} else{
			clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Página %d → Cache MISS", pid, pagina))
		}
	}
	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error al obtener marco: %s", err))
		return
	}
	
	consultaWrite(pid, marco, dato)

	if( globalsCpu.CpuConfig.CacheEntries > 0) {
		// Agregar a la caché
		cacheUtils.AgregarACache(pid, pagina, dato)
		clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Página %d → Agregando a caché", pid, pagina))
	}

	// Log
	clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Dir. lógica: %d → Dir. física: %d, Dato: %s", pid, direccionLogica, marco, dato))

}

//
func consultaWrite(pid int, marco int, dato string) {
	// Armar paquete y enviar
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		dato,
	}
	paquete := clientUtils.Paquete{Valores: valores}

	clientUtils.EnviarPaquete(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"writeMemoria",
		paquete,
	)
}

func consultaRead(pid int,marco int, direccionLogica int)(string,error){
// Armar paquete y enviar
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)
	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)

	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(desplazamiento),
	}
	paquete := clientUtils.Paquete{Valores: valores}

	respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paquete,
	)
	
	if respuesta == nil {
		clientUtils.Logger.Error(fmt.Sprintf("No se recibió respuesta de memoria para PID %d Página %d", pid, pagina))
		return "", fmt.Errorf("no se recibió respuesta de memoria")
	}

	contenido := string(respuesta)

	return contenido,nil
}

//-----------------------------

func LimpiarProceso(pid int) {
	//Paso los datos de la cache que fueron modificados a memoria
	// Luego limpio el cache y luego la TLB
	if globalsCpu.CpuConfig.CacheEntries > 0 {
		cacheUtils.FlushPaginasModificadas(pid)
		cacheUtils.LimpiarCache()
	}
	tlbUtils.LimpiarTLB()
}
