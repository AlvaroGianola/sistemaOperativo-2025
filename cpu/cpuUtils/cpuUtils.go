package cpuUtils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

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

	// Log obligatorio simulado: FETCH
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - FETCH - Program Counter: %d", datos.Pid, datos.Pc))
	w.WriteHeader(http.StatusOK)
}

// Envia handshake al Kernel con IP y puerto de esta CPU
func EnviarHandshakeAKernel(indentificador string, puertoLibre int) {

	puertoCpu := strconv.Itoa(puertoLibre)

	valores := []string{indentificador, globalsCpu.CpuConfig.IpCpu, puertoCpu}

	clientUtils.GenerarYEnviarPaquete(valores, globalsCpu.CpuConfig.IpKernel, globalsCpu.CpuConfig.PortKernel, "cpus") //IP y Puerto de la CPU

}

// handleProceso será el núcleo del ciclo de instrucción en Checkpoint 2 en adelante
// Por ahora queda como placeholder para mantener la estructura modular
func handleProceso(proceso Proceso) {
	// Aquí se implementará el ciclo: Fetch -> Decode -> Execute -> Check Interrupt
	// Por ahora solo lo dejamos declarado para usarlo desde RecibirProceso
	// Esto ayuda a mantener la arquitectura limpia y predecible
}

// Simula la recepción de una interrupción
func RecibirInterrupcion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("## Llega interrupción al puerto Interrupt")
	w.WriteHeader(http.StatusOK)
}
