package cpuUtils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

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

// globalsCpu.go (o donde tengas tus globals)
/*
var (
	ejecutando      = false
	ejecutandoMutex sync.Mutex
)*/

var PIDAnterior atomic.Int32
var UltimaSycall string
var mutexUltimaSycall sync.Mutex
var cancelProcesoActual context.CancelFunc
var ctxMutex sync.Mutex

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
		clientUtils.Logger.Error(fmt.Sprintf("error leyendo respuesta: %s", err.Error()))
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

	/*clientUtils.Logger.Info("Informacion de memoria obtenida correctamente",
		"Tamaño página", globalsCpu.Memoria.TamanioPagina,
		"Niveles", globalsCpu.Memoria.NivelesPaginacion,
		"Entradas por tabla", globalsCpu.Memoria.CantidadEntradas,
	)*/
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

	if pid != int(PIDAnterior.Load()) && UltimaSycall != "INIT_PROC" {
		globalsCpu.Interrupciones.ExisteInterrupcion = false
		globalsCpu.Interrupciones.Motivo = ""
	}
	// Creamos nuevo proceso local, sin depender de globalsCpu.ProcesoActual
	proceso := &globalsCpu.Proceso{
		Pid: pid,
		Pc:  pc,
	}

	clientUtils.Logger.Info(fmt.Sprintf("## Llega proceso - PID: %d, PC: %d", pid, pc))
	ctxMutex.Lock()
	// Cancelar ejecución anterior si había
	if cancelProcesoActual != nil {
		clientUtils.Logger.Warn("Cancelando ejecución de proceso anterior")
		cancelProcesoActual()
	}

	// Crear nuevo contexto para este proceso
	var ctx context.Context
	ctx, cancelProcesoActual = context.WithCancel(context.Background())
	ctxMutex.Unlock()

	go HandleProceso(ctx, proceso)
	w.WriteHeader(http.StatusOK)
}

func PedirSiguienteInstruccionMemoria(proceso *globalsCpu.Proceso) (string, bool) {
	valores := []string{strconv.Itoa(proceso.Pid), strconv.Itoa(proceso.Pc)}
	paquete := clientUtils.Paquete{Valores: valores}
	instruccion := clientUtils.EnviarPaqueteConRespuestaBody(globalsCpu.CpuConfig.IpMemory, globalsCpu.CpuConfig.PortMemory, "siguienteInstruccion", paquete)
	//clientUtils.Logger.Info(string(instruccion))
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
func HandleProceso(ctx context.Context, proceso *globalsCpu.Proceso) {
	for {
		select {
		case <-ctx.Done():
			clientUtils.Logger.Warn(fmt.Sprintf("## PID: %d - Cancelado por llegada de nuevo proceso", proceso.Pid))
			return
		default:
			// seguir normalmente
		}
		//#FETCH
		instruccion, ok := PedirSiguienteInstruccionMemoria(proceso)
		if !ok {

			clientUtils.Logger.Error("Error al pedir la siguiente instruccion a memoria")
			return
		}
		clientUtils.Logger.Info(fmt.Sprintf("## PID: %d - FETCH - Program Counter: %d", proceso.Pid, proceso.Pc))
		clientUtils.Logger.Info(fmt.Sprintf("## Instrucción: %s", instruccion))
		//#DECODE
		cod_op, variables := DecodeInstruccion(instruccion)
		clientUtils.Logger.Info(fmt.Sprintf("## Instrucción decodificada: %s, con las variables %s", cod_op, variables))
		//#EXECUTE
		//clientUtils.Logger.Info("## Ejecutando instrucción")
		cont := ExecuteInstruccion(proceso, cod_op, variables)

		clientUtils.Logger.Info("## Verificando interrupciones")
		if globalsCpu.Interrupciones.ExisteInterrupcion {
			clientUtils.Logger.Info("## Interrupcion recibida")
			globalsCpu.Interrupciones.ExisteInterrupcion = false
			EnviarResultadoAKernel(proceso.Pc, globalsCpu.Interrupciones.Motivo, nil)
			return
		}

		if !cont {
			if cod_op == EXIT {
				clientUtils.Logger.Info("## Proceso finalizado")
			}
			// salí SIEMPRE; no sigas chequeando interrupciones ni nada
			return
		}
		//#CHECK
		// le pregunto a kernel si hay una interrupción

	}

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

	//resultadoStr := strings.Join(valores, " ")
	//clientUtils.Logger.Info(fmt.Sprintf("valores a enviar a kernel: %s", resultadoStr))

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

func ExecuteInstruccion(proceso *globalsCpu.Proceso, cod_op string, variables []string) bool {
	clientUtils.Logger.Info(fmt.Sprintf("## PID: %d - Ejecutando: %s - %s", proceso.Pid, cod_op, variables))
	switch cod_op {
	case NOOP:
		//clientUtils.Logger.Info("## Ejecutando NOOP")
		//time.Sleep(2 * time.Second)
		proceso.Pc++
		return true

	case WRITE:
		//clientUtils.Logger.Info("## Ejecutando WRITE")
		direccion := variables[0]
		dato := variables[1]
		direccionInt, err := strconv.Atoi(direccion)

		if err != nil {
			clientUtils.Logger.Error("WRITE: argumento inválido, no es un número")
			return false
		}

		writeMemoria(proceso.Pid, direccionInt, dato)
		proceso.Pc++
		return true
	case READ:
		//clientUtils.Logger.Info("## Ejecutando READ")
		direccion := variables[0]
		tamanio := variables[1]
		direccionInt, err := strconv.Atoi(direccion)
		if err != nil {
			clientUtils.Logger.Error("READ: argumento inválido, no es un número")
			return false
		}
		tamanioInt, err := strconv.Atoi(tamanio)

		if err != nil {
			clientUtils.Logger.Error("READ: argumento inválido, no es un número")
			return false
		}

		readMemoria(proceso.Pid, direccionInt, tamanioInt)
		proceso.Pc++
		return true
	case GOTO:
		//clientUtils.Logger.Info("## Ejecutando GOTO")
		nuevoPC, err := strconv.Atoi(variables[0])
		if err != nil {
			clientUtils.Logger.Warn("GOTO: argumento inválido, no es un número")
			break
		}
		proceso.Pc = nuevoPC
		return true
	case IO, INIT_PROC, DUMP_MEMORY, EXIT:
		Syscall(proceso, cod_op, variables)
		return false // ← Esto evita volver al for
	default:
		clientUtils.Logger.Error("## Instruccion no reconocida")
		return false
	}
	return false
}

func Syscall(proceso *globalsCpu.Proceso, cod_op string, variables []string) {
	PIDAnterior.Store(int32(proceso.Pid))
	mutexUltimaSycall.Lock()
	UltimaSycall = cod_op
	mutexUltimaSycall.Unlock()
	switch cod_op {
	case IO:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar IO")
		LimpiarProceso(proceso.Pid)
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		return
	case INIT_PROC:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar INIT_PROC")
		LimpiarProceso(proceso.Pid)
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		return
	case DUMP_MEMORY:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar DUMP_MEMORY")
		LimpiarProceso(proceso.Pid)
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		return
	case EXIT:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar EXIT")
		LimpiarProceso(proceso.Pid)
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		return
	default:
		clientUtils.Logger.Error("Error, instruccion no reconocida")
		return
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
			if desplazamiento+tamanio <= len(contenidoStr) {
				fmt.Println(contenidoStr[desplazamiento : desplazamiento+tamanio])
			} else {
				clientUtils.Logger.Error("READ - Error: el rango solicitado excede el contenido en caché")
				fmt.Println(contenidoStr[desplazamiento:])
			}
			return
		} else {
			//clientUtils.Logger.Info(fmt.Sprintf("READ - PID: %d, Página %d → Cache MISS", pid, pagina))
		}
	}

	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("READ - Error al obtener marco: %s", err))
		return
	}

	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - OBTENER MARCO - Página: %d - Marco: %d", pid, pagina, marco))

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
		clientUtils.Logger.Error("READ - Error, el tamaño a leer es mayor a su contenido, se devolverá el contenido completo")
		fmt.Println(string(contenido))
		return
	}

	fmt.Println(string(contenido[:tamanio]))
}

func writeMemoria(pid int, direccionLogica int, dato string) {
	// Traducir dirección lógica a física
	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)

	if globalsCpu.CpuConfig.CacheEntries != 0 {
		_, encontroDato := cacheUtils.BuscarPaginaEnCache(pid, pagina)
		if encontroDato {
			//Logs en buscarPaginaEnCache
			clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache HIT - Página %d, Contenido %s", pid, pagina, dato))
			err := cacheUtils.ModificarContenidoCache(pid, pagina, dato, direccionLogica)
			if err != nil {
				clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error al modificar contenido en cache: %s", err))
				return
			} else {
				return
			}
		} else {
			clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache MISS - Página %d", pid, pagina))
			cacheUtils.AgregarACache(pid, direccionLogica, []byte(dato))
			return
		}
	}

	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("WRITE - Error al obtener marco: %s", err))
		return
	}

	consultaWrite(pid, marco, direccionLogica, []byte(dato))

	//Log
	//clientUtils.Logger.Info(fmt.Sprintf("PID: %d - OBTENER MARCO - Página: %d - Marco: %d", proceso.Pid, pagina, marco))

	if globalsCpu.CpuConfig.CacheEntries > 0 {
		// Agregar a la caché
		cacheUtils.AgregarACache(pid, direccionLogica, []byte(dato))
		clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache Add - Página %d", pid, pagina))
	}

	// Log
	clientUtils.Logger.Info(fmt.Sprintf("“PID: %d - Acción: Escribir - Dirección Física: %d - Valor: %s.", pid, marco, dato))

}

func consultaWrite(pid int, marco int, direccionLogica int, datos []byte) error {
	pageSize := globalsCpu.Memoria.TamanioPagina
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)

	if len(datos) == 1 {
		valores := []string{
			strconv.Itoa(pid),
			strconv.Itoa(marco*pageSize + desplazamiento),
			strconv.Itoa(int(datos[0])),
		}
		paquete := clientUtils.Paquete{Valores: valores}

		respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"writeMemoria",
			paquete,
		)

		if respuesta == nil {
			return fmt.Errorf("error al escribir valor en memoria")
		}

		return nil
	}

	if desplazamiento+len(datos) > pageSize {
		return fmt.Errorf("datos a escribir exceden tamaño de página")
	}

	// Primero leemos toda la página para modificar solo los bytes necesarios
	valoresLeer := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	paqueteLeer := clientUtils.Paquete{Valores: valoresLeer}

	paginaCompleta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paqueteLeer,
	)

	if len(paginaCompleta) != pageSize {
		return fmt.Errorf("no se pudo leer página completa antes de escribir")
	}

	// Modificar solo el rango correspondiente
	copy(paginaCompleta[desplazamiento:], datos)

	// Preparar paquete para escribir página completa
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	for _, b := range paginaCompleta {
		valores = append(valores, strconv.Itoa(int(b)))
	}
	paquete := clientUtils.Paquete{Valores: valores}

	clientUtils.EnviarPaquete(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"writePagina",
		paquete,
	)

	return nil
}

func consultaRead(pid int, marco int, direccionLogica int, tamanio int) ([]byte, error) {
	pageSize := globalsCpu.Memoria.TamanioPagina
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)

	if tamanio == 1 {
		valores := []string{
			strconv.Itoa(pid),
			strconv.Itoa(marco*pageSize + desplazamiento),
		}
		paquete := clientUtils.Paquete{Valores: valores}

		respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"readMemoria",
			paquete,
		)

		if respuesta == nil {
			return nil, fmt.Errorf("valor recibido nulo")
		}
		return respuesta, nil
	}

	// Armar paquete para leer página completa
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	paquete := clientUtils.Paquete{Valores: valores}

	respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paquete,
	)

	if len(respuesta) != pageSize {
		clientUtils.Logger.Error(fmt.Sprintf("READ - Tamaño de página recibido incorrecto: esperado %d, recibido %d", pageSize, len(respuesta)))
		return nil, fmt.Errorf("tamaño de página recibido incorrecto")
	}

	// Luego, retornar sólo los bytes solicitados (con desplazamiento)
	if desplazamiento+tamanio > pageSize {
		return nil, fmt.Errorf("rango de lectura excede tamaño de página")
	}

	return respuesta[desplazamiento : desplazamiento+tamanio], nil
}

//-----------------------------

func LimpiarProceso(pid int) {
	//Paso los datos de la cache que fueron modificados a memoria
	// Luego limpio el cache y luego la TLB
	if globalsCpu.CpuConfig.CacheEntries != 0 {
		//clientUtils.Logger.Info("Cache detectada, se flushearan las paginas modificadas a la memoria")
		cacheUtils.FlushPaginasModificadas(pid)
		cacheUtils.LimpiarCache()
	}
	if globalsCpu.CpuConfig.TlbEntries != 0 {
		tlbUtils.LimpiarTLB()
	}
}
