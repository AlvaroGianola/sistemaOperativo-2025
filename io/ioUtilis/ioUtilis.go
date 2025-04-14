package ioUtils

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

type Config struct {
	IPKernel     string `json:"ip_kernel"`
	PuertoKernel int    `json:"port_kernel"`
	PortIO       int    `json:"port_io"`
	LogLevel     string `json:"log_level"`
}

var IoConfig *Config

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

func RecibirPeticion(w http.ResponseWriter, r *http.Request)

func EnviarHandshakeAKernel(nombre string) {
	ipIO, err := clientUtils.ObtenerIPLocal()
	if err != nil {
		log.Printf("Error al obtener ip de la Io")
		return
	}

	puertoIo := strconv.Itoa(IoConfig.PortIO)

	valores := []string{nombre, ipIO, puertoIo}

	clientUtils.GenerarYEnviarPaquete(valores, IoConfig.IPKernel, IoConfig.PuertoKernel, "ios") //IP y Puerto de la CPU

}
