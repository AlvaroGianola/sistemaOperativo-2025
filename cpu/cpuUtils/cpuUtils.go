package cpuUtils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	cacheUtils "github.com/sisoputnfrba/tp-golang/cpu/cache"
	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	globalscpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
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
	respuesta := clientUtils.EnviarPaqueteConRespuesta(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"obtenerConfiguracionMemoria",
		clientUtils.Paquete{},
	)

	if respuesta == nil {
		// Manejar error
		return
	}
	defer respuesta.Body.Close()

	// Leer el body de la respuesta
	bodyBytes, err := io.ReadAll(respuesta.Body)
	if err != nil {
		clientUtils.Logger.Error("error leyendo respuesta: %s", err.Error())
		return
	}

	// Si esperás un JSON, podés deserializarlo así:
	var valores struct {
		TamanioPagina    int `json:"tamanioPagina"`
		Niveles          int `json:"niveles"`
		EntradasPorNivel int `json:"entradasPorNivel"`
	}

	err = json.Unmarshal(bodyBytes, &valores)
	if err != nil {
		clientUtils.Logger.Error("error decodificando respuesta", "error", err.Error())
		return
	}

	// Ahora 'valores' contiene los datos de la respuesta

	globalsCpu.Memoria.TamanioPagina = valores.TamanioPagina
	globalsCpu.Memoria.NivelesPaginacion = valores.Niveles
	globalsCpu.Memoria.CantidadEntradas = valores.EntradasPorNivel

	clientUtils.Logger.Info("Informacion de memoria obtenida correctamente",
		"Tamaño página", globalsCpu.Memoria.TamanioPagina,
		"Niveles", globalsCpu.Memoria.NivelesPaginacion,
		"Entradas por tabla", globalscpu.Memoria.CantidadEntradas,
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
	globalsCpu.Interrupciones.ExisteInterrupcion = false
	globalsCpu.Interrupciones.Motivo = ""
	HandleProceso(globalsCpu.ProcesoActual)

	w.WriteHeader(http.StatusOK)
}

func PedirSiguienteInstruccionMemoria() (string, bool) {
	clientUtils.Logger.Info("estoy en pedirInstrucMem")
	valores := []string{strconv.Itoa(globalsCpu.ProcesoActual.Pid), strconv.Itoa(globalsCpu.ProcesoActual.Pc)}
	paquete := clientUtils.Paquete{Valores: valores}
	instruccion := clientUtils.EnviarPaqueteConRespuestaBody(globalsCpu.CpuConfig.IpMemory, globalsCpu.CpuConfig.PortMemory, "siguienteInstruccion", paquete)
	clientUtils.Logger.Info(string(instruccion))
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
	clientUtils.Logger.Info("estoy en handleproc")
	for {

		//#FETCH
		instruccion, ok := PedirSiguienteInstruccionMemoria()
		if !ok {

			clientUtils.Logger.Error("Error al pedir la siguiente instruccion a memoria")
			break
		}
		clientUtils.Logger.Info(fmt.Sprintf("## PID: %d - FETCH - Program Counter: %d\n", globalsCpu.ProcesoActual.Pid, globalsCpu.ProcesoActual.Pc))
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
				clientUtils.Logger.Info("## Llega interrupción al puerto Interrupt")
				break
			}
		}

		// Si la instrucción es EXIT o INVALIDA, salimos del ciclo
		if cod_op == EXIT {
			clientUtils.Logger.Info("## Proceso finalizado")
			break
		} else if cod_op == IO || cod_op == INIT_PROC || cod_op == DUMP_MEMORY {
			break
		}
		if cod_op == INVALID {
			clientUtils.Logger.Error("## Instrucción inválida, abortando ejecución")
			break
		}
		if globalsCpu.Interrupciones.ExisteInterrupcion {
			clientUtils.Logger.Info("## Interrupcion recibida")
			EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, globalsCpu.Interrupciones.Motivo, nil)
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

func RecibirInterrupcion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("## Llega interrupción al puerto Interrupt")
	paquete := serverUtils.RecibirPaquetes(w, r)
	globalsCpu.Interrupciones.ExisteInterrupcion = true
	globalsCpu.Interrupciones.Motivo = paquete.Valores[0]
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
	clientUtils.Logger.Info(fmt.Sprintf("## PID: %d - Ejecutando: %s - %s\n", globalsCpu.ProcesoActual.Pid, cod_op, variables))
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
		LimpiarProceso(globalsCpu.ProcesoActual.Pid)
		globalsCpu.ProcesoActual.Pc++
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	case INIT_PROC:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar INIT_PROC")
		LimpiarProceso(globalsCpu.ProcesoActual.Pid)
		globalsCpu.ProcesoActual.Pc++
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	case DUMP_MEMORY:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar DUMP_MEMORY")
		LimpiarProceso(globalsCpu.ProcesoActual.Pid)
		globalsCpu.ProcesoActual.Pc++
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	case EXIT:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar EXIT")
		LimpiarProceso(globalsCpu.ProcesoActual.Pid)
		EnviarResultadoAKernel(globalsCpu.ProcesoActual.Pc, cod_op, variables)
	default:
		clientUtils.Logger.Error("Error, instruccion no reconocida")
	}
}

// Escribir y Leer memoria
// 1-Bucar el nro de pagina
// 2-Buscar en la cache si existe
// 3-Si no existe, buscar en la tlb
// 4-Si no existe en la tlb, buscar en memoria
// 5-Escribir o leer el contenido
func readMemoria(pid int, direccionLogica int, tamanio int) {

	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)
	if globalsCpu.CpuConfig.CacheEntries > 0 {
		contenido, encontroContenido := cacheUtils.BuscarPaginaEnCache(pid, pagina)
		if encontroContenido {
			contenidoStr := string(contenido)
			fmt.Println(contenidoStr[desplazamiento : tamanio+desplazamiento])
			return
		} else {
			clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d → Cache MISS", pid, pagina))
		}
	}
	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("READ - Error al obtener marco: %s", err))
		return
	}

	//Log
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - OBTENER MARCO - Página: %d - Marco: %d", globalsCpu.ProcesoActual.Pid, pagina, marco))

	contenido, err := consultaRead(pid, marco, direccionLogica, tamanio)

	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("READ - Error al consultar memoria: %s", err))
		return
	}

	if globalsCpu.CpuConfig.CacheEntries > 0 {
		cacheUtils.AgregarACache(pid, direccionLogica, contenido)
		clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d → Agregando a caché", pid, pagina))
	}

	if tamanio > len(contenido) {
		clientUtils.Logger.Error("Error, el tamaño a leer en la direccion es mayor a su contenido, se devolvera el contenido completo")
		fmt.Println(contenido)
		return
	}

	//Log
	clientUtils.Logger.Info(fmt.Sprintf("“PID: %d - Acción: READ - Dirección Física: %d - Valor: %s.\n", pid, marco, contenido[:globalsCpu.Memoria.TamanioPagina]))
	fmt.Println(contenido[:tamanio])

}

func writeMemoria(pid int, direccionLogica int, dato string) {
	// Traducir dirección lógica a física
	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)

	if globalsCpu.CpuConfig.CacheEntries > 0 {
		_, encontroDato := cacheUtils.BuscarPaginaEnCache(pid, pagina)
		if encontroDato {
			clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Página %d, Contenido %s → Cache HIT\n", pid, pagina, dato))
			err := cacheUtils.ModificarContenidoCache(pid, pagina, dato, direccionLogica)
			if err != nil {
				clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error al modificar contenido en cache: %s\n", err))
				return
			} else {
				return
			}
		} else {
			clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Página %d → Cache MISS", pid, pagina))
		}
	}
	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error al obtener marco: %s", err))
		return
	}

	//Log
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - OBTENER MARCO - Página: %d - Marco: %d", globalsCpu.ProcesoActual.Pid, pagina, marco))

	consultaWrite(pid, marco, []byte(dato), direccionLogica)

	if globalsCpu.CpuConfig.CacheEntries > 0 {
		// Agregar a la caché
		cacheUtils.AgregarACache(pid, direccionLogica, []byte(dato))
		clientUtils.Logger.Info(fmt.Sprintf("WRITE - PID: %d, Página %d → Agregando a caché", pid, pagina))
	}

	// Log
	clientUtils.Logger.Info(fmt.Sprintf("“PID: %d - Acción: WRITE - Dirección Física: %d - Valor: %s.\n", pid, marco, dato))

}

func consultaWrite(pid int, marco int, dato []byte, direccionLogica int) {
	// Armar paquete y enviar

	if len(dato)+direccionLogica > globalsCpu.Memoria.TamanioPagina {
		clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error: el dato %s excede el tamaño de la página", string(dato)))
		return
	}

	for i := 0; i < len(dato); i++ {
		desplazamiento := mmuUtils.ObtenerDesplazamiento(i + direccionLogica)

		valores := []string{
			strconv.Itoa(pid),
			strconv.Itoa(marco),
			strconv.Itoa(desplazamiento),
			string(dato[i]),
		}
		paquete := clientUtils.Paquete{Valores: valores}

		clientUtils.GenerarYEnviarPaquete(
			paquete.Valores,
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"writeMemoria",
		)
	}

}

func consultaRead(pid int, marco int, direccionLogica int, tamanio int) ([]byte, error) {
	// Armar paquete y enviar
	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)
	contenido := make([]byte, globalsCpu.Memoria.TamanioPagina)

	for i := 0; i < tamanio; i++ {
		desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica + i)

		if direccionLogica+i > globalsCpu.Memoria.TamanioPagina {
			clientUtils.Logger.Error(fmt.Sprintf("READ - Error: el desplazamiento %d está fuera del rango de la página", desplazamiento))
			return nil, fmt.Errorf("desplazamiento fuera de rango")
		}

		valores := []string{
			strconv.Itoa(pid),
			strconv.Itoa(marco),
			strconv.Itoa(desplazamiento),
		}
		paquete := clientUtils.Paquete{Valores: valores}

		respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"readMemoria",
			paquete,
		)

		if respuesta == nil {
			clientUtils.Logger.Error(fmt.Sprintf("No se recibió respuesta de memoria para PID %d Página %d", pid, pagina))
			return nil, fmt.Errorf("no se recibió respuesta de memoria")
		} else {
			contenido[desplazamiento] = respuesta[0]
		}
	}
	return contenido, nil
}

//-----------------------------

func LimpiarProceso(pid int) {
	//Paso los datos de la cache que fueron modificados a memoria
	// Luego limpio el cache y luego la TLB
	if globalsCpu.CpuConfig.CacheEntries > 0 {
		clientUtils.Logger.Info("Cache detectada, se flushearan las paginas modificadas a la memoria")
		cacheUtils.FlushPaginasModificadas(pid)
		cacheUtils.LimpiarCache()
	}
	tlbUtils.LimpiarTLB()
}
