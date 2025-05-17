package cpuUtils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
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

// Inicializa la configuración leyendo el archivo json indicado
func IniciarConfiguracion(filePath string) *globalsCpu.Config {
	var config *globalsCpu.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

// Representa un proceso con su PID y su Program Counter (PC)
type Proceso struct {
	Pid int `json:"pid"`
	Pc  int `json:"pc"`
}

// Recibe un proceso del Kernel y lo loguea
func RecibirProceso(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error leyendo body: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var datos Proceso
	err = json.Unmarshal(body, &datos)
	if err != nil {
		log.Printf("Error parseando JSON: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	clientUtils.Logger.Info("## Llega proceso al puerto CPU")
	w.WriteHeader(http.StatusOK)
	clientUtils.Logger.Info(fmt.Sprintf("## PID: %d, PC: %d", datos.Pid, datos.Pc))
	HandleProceso(datos)

}

// Envia handshake al Kernel con IP y puerto de esta CPU
func EnviarHandshakeAKernel(indentificador string, puertoLibre int) {

	puertoCpu := strconv.Itoa(puertoLibre)

	valores := []string{indentificador, globalsCpu.CpuConfig.IpCpu, puertoCpu}

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "cpus") //IP y Puerto de la CPU

}

// handleProceso será el núcleo del ciclo de instrucción en Checkpoint 2 en adelante
// Por ahora queda como placeholder para mantener la estructura modular
func HandleProceso(proceso Proceso) {

	for {
		//#FETCH
		instruccion := globalsCpu.ObtenerMix(proceso.Pc, proceso.Pid)
		clientUtils.Logger.Info(fmt.Sprintf("## Instrucción: %s", instruccion))
		//#DECODE
		cod_op, variables := DecodeInstruccion(instruccion)
		clientUtils.Logger.Info(fmt.Sprintf("## Instrucción decodificada: %s, con las variables %s", cod_op, variables))
		//#EXECUTE
		clientUtils.Logger.Info("## Ejecutando instrucción")
		ExecuteInstruccion(&proceso, cod_op, variables)
		//#CHECK
		if cod_op == GOTO || cod_op == EXIT {
			break
		}
		// Aquí se implementará el ciclo: Fetch -> Decode -> Execute -> Check Interrupt
		// Por ahora solo lo dejamos declarado para usarlo desde RecibirProceso
		// Esto ayuda a mantener la arquitectura limpia y predecible
	}

}

// Simula la recepción de una interrupción
func RecibirInterrupcion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("## Llega interrupción al puerto Interrupt")
	w.WriteHeader(http.StatusOK)
}

//----------------------------------------------------------------------
/*
func EnviarResultadoAKernel(pid int, motivo string) {

	url := fmt.Sprintf("http://%s:%d/resultadoProcesos", globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel)

	body := map[string]interface{}{
		"pid":    pid,
		"motivo": motivo,
	}

	jsonData, _ := json.Marshal(body)
	_, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("Fallo al notificar al Kernel: %s", err.Error()))
	}
}
*/

func EnviarResultadoAKernel(pc int, cod_op string, args []string) {
	pcStr := strconv.Itoa(pc)

	ids := []string{globalsCpu.Identificador, pcStr}

	valores := append(ids, args...)

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "/resultadoProcesos")
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

func ExecuteInstruccion(proceso *Proceso, cod_op string, variables []string) {
	switch cod_op {
	case NOOP:
		clientUtils.Logger.Info("## Ejecutando NOOP")
		time.Sleep(2)
		proceso.Pc++
	case WRITE:
		clientUtils.Logger.Info("## Ejecutando WRITE")
		WriteFile(proceso.Pid, variables[0], variables[1])
		proceso.Pc++
	case READ:
		clientUtils.Logger.Info("## Ejecutando READ")
		ReadFile(proceso.Pid, variables[0], 20)
		proceso.Pc++
	case GOTO:
		clientUtils.Logger.Info("## Ejecutando GOTO")
		proceso.Pc = 0
	default:
		if cod_op != IO && cod_op != INIT_PROC && cod_op != DUMP_MEMORY && cod_op != EXIT {
			clientUtils.Logger.Error("## Instruccion no reconocida")
		} else {
			Syscall(proceso, cod_op, variables)
		}
	}

}

func Syscall(proceso *Proceso, cod_op string, variables []string) {
	switch cod_op {
	case IO:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar IO")
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		proceso.Pc++
	case INIT_PROC:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar INIT_PROC")
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		proceso.Pc++
	case DUMP_MEMORY:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar DUMP_MEMORY")
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		proceso.Pc++
	case EXIT:
		clientUtils.Logger.Info("## Llamar al sistema para ejecutar EXIT")
		EnviarResultadoAKernel(proceso.Pc, cod_op, variables)
		proceso.Pc++
	default:
		clientUtils.Logger.Error("Error, instruccion no reconocida")
	}
}

func ReadFile(pid int, path string, lineCount int) {
	file, err := os.Open(path)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("PID: %d - Error al abrir archivo para READ: %s", pid, err.Error()))
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		clientUtils.Logger.Info(fmt.Sprintf("PID: %d - LECTURA - Línea %d: %s", pid, count+1, scanner.Text()))
		count++
		if count >= lineCount {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("PID: %d - Error al leer archivo: %s", pid, err.Error()))
	}
}

func WriteFile(pid int, path string, data string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("PID: %d - Error al abrir archivo para WRITE: %s", pid, err.Error()))
		return
	}
	defer file.Close()

	_, err = file.WriteString(data + "\n")
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("PID: %d - Error al escribir archivo: %s", pid, err.Error()))
		return
	}
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - ESCRITURA - Valor escrito: %s", pid, data))
}
