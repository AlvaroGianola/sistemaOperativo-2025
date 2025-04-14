package cpuUtils

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

type Config struct {
	PortCpu         int    `json:"port_cpu"`
	IpMemory        string `json:"ip_memory"`
	PortMemory      int    `json:"port_memory"`
	IpKernel        string `json:"ip_kernel"`
	PortKernel      int    `json:"port_kernel"`
	TlbEntries      int    `json:"tlb_entries"`
	TlbReplacement  string `json:"tlb_replacement"`
	CacheEntries    int    `json:"cache_entries"`
	CacheReplacment string `json:"cache_replacement"`
	CacheDelay      int    `json:"cache_delay"`
	LogLevel        string `json:"log_level"`
}

var CpuConfig *Config

func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

type Proceso struct {
	Pid int `json:"pid"`
	Pc  int `json:"pc"`
}

func RecibirProceso(w http.ResponseWriter, r *http.Request) {

	var paqueteRecibido serverUtils.Paquete = serverUtils.RecibirPaquetes(w, r)
	pid, err1 := strconv.Atoi(paqueteRecibido.Valores[0])
	if err1 != nil {
		log.Printf("Error al convertir pid a int")
		return
	}
	pc, err2 := strconv.Atoi(paqueteRecibido.Valores[1])
	if err2 != nil {
		log.Printf("Error al convertir pc a int")
		return
	}
	var nuevoProceso Proceso = Proceso{pid, pc}
	handleProceso(nuevoProceso)
}

func EnviarHandshakeAKernel() {

	ipCpu, err := clientUtils.ObtenerIPLocal()
	if err != nil {
		log.Printf("Error al obtener ip de la CPU")
		return
	}

	puertoCpu := strconv.Itoa(CpuConfig.PortCpu)

	valores := []string{ipCpu, puertoCpu}

	clientUtils.GenerarYEnviarPaquete(valores, CpuConfig.IpKernel, CpuConfig.PortKernel, "cpus") //IP y Puerto de la CPU

}

func handleProceso(proceso Proceso) {

}

func RecibirInterrupcion(w http.ResponseWriter, r *http.Request)
