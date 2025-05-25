package cpuUtils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
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
	instruccion := clientUtils.EnviarPaqueteConRespuestaBody(globalsCpu.CpuConfig.IpMemory, globalsCpu.CpuConfig.PortMemory, "recibirInstruccionMemoria", paquete)

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
		//TODO
	}

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
			clientUtils.Logger.Error("Cantidad de parametros recibidos en la instruccion %s incorrecto, no se deben ingresar parametros para esta instruccion", cod_op)
		}
	case GOTO:
		cod_op = GOTO
		if len(variables) != 1 {
			clientUtils.Logger.Error("Cantidad de parametros recibidos en la instruccion GOTO incorrecto, se debe ingresar 1 parametro")
		}
	case READ, WRITE, IO, INIT_PROC:
		if len(variables) != 2 {
			clientUtils.Logger.Error("Cantidad de parametros recibidos en la instruccion %s incorrecto, se deben ingresar 2 parametros", cod_op)
		}
	default:
		if len(variables) > 3 {
			clientUtils.Logger.Error("Instrucción inválida")
			cod_op = INVALID
		}
	}
	return cod_op, variables
}

func ExecuteInstruccion(proceso *globalsCpu.Proceso, cod_op string, variables []string) {
	switch cod_op {
	case NOOP:
		clientUtils.Logger.Info("## Ejecutando NOOP")
		time.Sleep(2 * time.Second)
		proceso.Pc++
	case WRITE:
		clientUtils.Logger.Info("## Ejecutando WRITE")
		direccion := variables[0]
		dato := variables[1]
		writeMemoria(proceso.Pid, direccion, dato)
		proceso.Pc++
	case READ:
		clientUtils.Logger.Info("## Ejecutando READ")
		direccion := variables[0]
		tamanio := variables[1]
		readMemoria(proceso.Pid, direccion, tamanio)
		proceso.Pc++
	case GOTO:
		clientUtils.Logger.Info("## Ejecutando GOTO")
		nuevoPC, err := strconv.Atoi(variables[0])
		if err != nil {
			clientUtils.Logger.Warn("GOTO: argumento inválido, no es un número")
			break
		}
		proceso.Pc = nuevoPC
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
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
	case INIT_PROC:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar INIT_PROC")
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
	case DUMP_MEMORY:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar DUMP_MEMORY")
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
	case EXIT:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar EXIT")
		proceso.Pc++
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
	default:
		clientUtils.Logger.Error("Error, instruccion no reconocida")
	}
}

func readMemoria(pid int, direccion string, tamanio string) {
	// Simula leer desde memoria: loguea y muestra por pantalla
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - LECTURA - Dirección lógica: %s, Tamaño: %s", pid, direccion, tamanio))
	fmt.Printf("[PID %d] Lectura desde memoria - Dirección: %s, Tamaño: %s\n", pid, direccion, tamanio)
}

func writeMemoria(pid int, direccion string, dato string) {
	// Simula escribir en memoria: loguea la operación
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - ESCRITURA - Dirección lógica: %s, Dato: %s", pid, direccion, dato))
	fmt.Printf("[PID %d] Escritura en memoria - Dirección: %s, Dato: %s\n", pid, direccion, dato)
}
