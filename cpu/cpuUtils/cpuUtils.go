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

	handleProceso(datos,w)

}

// Envia handshake al Kernel con IP y puerto de esta CPU
func EnviarHandshakeAKernel(indentificador string, puertoLibre int) {

	puertoCpu := strconv.Itoa(puertoLibre)

	valores := []string{indentificador, globalsCpu.CpuConfig.IpCpu, puertoCpu}

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "cpus") //IP y Puerto de la CPU

}

// handleProceso será el núcleo del ciclo de instrucción en Checkpoint 2 en adelante
// Por ahora queda como placeholder para mantener la estructura modular
func handleProceso(proceso Proceso,w http.ResponseWriter) {

	//#FETCH
	clientUtils.Logger.Info("## Llega proceso al puerto CPU")
	clientUtils.Logger.Info(fmt.Sprintf("## PID: %d, PC: %d", proceso.Pid, proceso.Pc))
	clientUtils.Logger.Info(fmt.Sprintf("## Instrucción: %s", solicitarProcesoMemoria(proceso.Pid, proceso.Pc)))
	//#DECODE
	cod_op,variables := decodeProceso(solicitarProcesoMemoria(proceso.Pid, proceso.Pc))
	clientUtils.Logger.Info(fmt.Sprintf("## Instrucción decodificada: %s, con las variables %s", cod_op, variables))
	w.WriteHeader(http.StatusOK)
	//#EXECUTE
	clientUtils.Logger.Info("## Ejecutando instrucción")
	executeInstruccion(&proceso, cod_op, variables)
	//#CHECK 
	

	// Aquí se implementará el ciclo: Fetch -> Decode -> Execute -> Check Interrupt
	// Por ahora solo lo dejamos declarado para usarlo desde RecibirProceso
	// Esto ayuda a mantener la arquitectura limpia y predecible
}

// Simula la recepción de una interrupción
func RecibirInterrupcion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("## Llega interrupción al puerto Interrupt")
	w.WriteHeader(http.StatusOK)
}

//----------------------------------------------------------------------

func solicitarProcesoMemoria(pid int, pc int) (string) {
	// Simula la solicitud de un proceso a la memoria
	// En este caso, simplemente devuelve el PID y un motivo vacío
	switch pc {
	case 0:
		return "NOOP"
	case 1:
		return "WRITE 0 EJEMPLO_DE_ENUNCIADO"
	case 2:
		return "READ 0 20"
	case 3:
		return "GOTO 0"
	case 4:
		return "IO IMPRESORA 2500"
	case 5:
		return "INIT_PROC proceso_1 256"
	case 6:
		return "DUMP_MEMORY"
	case 7:
		return "EXIT"
	default:
		return "NOOP"
	}

}

func decodeProceso(instruccion string) (cod_op string,variables []string) {
	instruccionPartida := strings.Split(instruccion, " ")

	if len(instruccionPartida) < 1 {
		variables = []string{}
		switch {
		case instruccionPartida[0] == "NOOP":
			cod_op = "NOOP"

		case instruccionPartida[0] == "EXIT":
			cod_op = "EXIT"
		
		case instruccionPartida[0] == "DUMP_MEMORY":
			cod_op = "DUMP_MEMORY"
		}
	}

	if len(instruccionPartida) < 2 {
		if (instruccionPartida[0] == "GOTO") {
			cod_op = "GOTO"
			variables = []string{instruccionPartida[1]}
		} else {
			cod_op = "NOOP"
			variables = []string{}
		}
	}

	if len(instruccionPartida) < 3 {
		variables = []string{instruccionPartida[1], instruccionPartida[2]}
		switch{
		case instruccionPartida[0] == "READ":
			cod_op = "READ"
		case instruccionPartida[0] == "WRITE":
			cod_op = "WRITE"
		case instruccionPartida[0] == "IO":
			cod_op = "IO"
		case instruccionPartida[0] == "INIT_PROC":
			cod_op = "INIT_PROC"
	
		} 
	}

	return cod_op, variables

}

func executeInstruccion(proceso *Proceso, cod_op string, variables []string) {
	switch cod_op {
		case "NOOP":
			clientUtils.Logger.Info("## Ejecutando NOOP")
			time.Sleep(500 * time.Millisecond)
		case "WRITE":
			clientUtils.Logger.Info("## Ejecutando WRITE")
			writeFile(proceso.Pid, variables[0], variables[1])
		case "READ":
			clientUtils.Logger.Info("## Ejecutando READ")
			readFile(proceso.Pid, variables[0], 20)
		case "GOTO":
			clientUtils.Logger.Info("## Ejecutando GOTO")
			proceso.Pc = 0
		default:
	}

	switch cod_op {
		case "IO":
			clientUtils.Logger.Info("## Ejecutando IO")
		case "INIT_PROC":
			clientUtils.Logger.Info("## Ejecutando INIT_PROC")
		case "DUMP_MEMORY":
			clientUtils.Logger.Info("## Ejecutando DUMP_MEMORY")
		case "EXIT":
			clientUtils.Logger.Info("## Ejecutando EXIT")
		default:
			clientUtils.Logger.Info("## Instrucción no reconocida")
	}

}

func readFile(pid int, path string, lineCount int) {
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

func writeFile(pid int, path string, data string) {
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