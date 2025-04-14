package kernelUtils

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

type Config struct {
	IpMemory           string `json:"ip_memory"`
	PortMemory         int    `json:"port_memory"`
	PortKernel         int    `json:"port_kernel"`
	SchedulerAlgorithm string `json:"scheduler_algorithm"`
	NewAlgorithm       string `json:"new_algorithm"`
	Alpha              string `json:"alpha"`
	SuspensionTime     int    `json:"suspension_time"`
	LogLevel           string `json:"log_level"`
}

var KernelConfig *Config

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

type Cpu struct {
	Ip     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

type Io struct {
	Nombre string
	Ip     string
	Puerto int
}

var cpusRegistradas []Cpu
var iosRegistradas []Io

// handshake
func RegistrarCpu(w http.ResponseWriter, r *http.Request) {

	var paqueteRecibido serverUtils.Paquete = serverUtils.RecibirPaquetes(w, r)
	puerto, err := strconv.Atoi(paqueteRecibido.Valores[1])
	if err != nil {
		log.Printf("Error al convertir paquete recibido a string")
		return
	}
	var nuevaCpu Cpu = Cpu{paqueteRecibido.Valores[0], puerto}
	cpusRegistradas = append(cpusRegistradas, nuevaCpu)
	log.Printf("CPU registrada: %+v", nuevaCpu)
}

func ResultadoProcesos(w http.ResponseWriter, r *http.Request)

func RegistrarIo(w http.ResponseWriter, r *http.Request) {

	var paqueteRecibido serverUtils.Paquete = serverUtils.RecibirPaquetes(w, r)
	puerto, err := strconv.Atoi(paqueteRecibido.Valores[2])
	if err != nil {
		log.Printf("Error al convertir el puerto del paquete recibido a int")
		return
	}

	var nuevaIo Io = Io{paqueteRecibido.Valores[0], paqueteRecibido.Valores[1], puerto}
	iosRegistradas = append(iosRegistradas, nuevaIo)
	//log.Print("IO registrada %+v", nuevaIo)
}
