package kernelUtils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	globalsKernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func IniciarConfiguracion(filePath string) *globalsKernel.Config {
	var config *globalsKernel.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

// Estructura para representar CPUs e IOs conectados al Kernel
type Cpu struct {
	Ip     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

type Io struct {
	Nombre string
	Ip     string
	Puerto int
}

// Listas globales para almacenar las CPUs e IOs conectadas
var cpusRegistradas []Cpu
var iosRegistradas []Io

// RegistrarCpu maneja el handshake de una CPU
// Espera recibir un JSON con formato ["ip", "puerto"]
func RegistrarCpu(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)

	puerto, err := strconv.Atoi(paquete.Valores[1])
	if err != nil {
		clientUtils.Logger.Info("Error al parsear puerto de CPU")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nuevaCpu := Cpu{
		Ip:     paquete.Valores[0],
		Puerto: puerto,
	}

	cpusRegistradas = append(cpusRegistradas, nuevaCpu)
	clientUtils.Logger.Info(fmt.Sprintf("CPU registrada: %+v", nuevaCpu))
	w.WriteHeader(http.StatusOK)
}

// ResultadoProcesos es un endpoint placeholder para futuras devoluciones de la CPU
func ResultadoProcesos(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("Recibido resultado de proceso (placeholder Checkpoint 1)")
	w.WriteHeader(http.StatusOK)
}


// RegistrarIo maneja el handshake de una IO
// Espera recibir un JSON con formato ["nombre", "ip", "puerto"]
func RegistrarIo(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)

	puerto, err := strconv.Atoi(paquete.Valores[2])
	if err != nil {
		clientUtils.Logger.Info("Error al parsear puerto de IO")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nuevaIo := Io{
		Nombre: paquete.Valores[0],
		Ip:     paquete.Valores[1],
		Puerto: puerto,
	}

	iosRegistradas = append(iosRegistradas, nuevaIo)
	clientUtils.Logger.Info(fmt.Sprintf("IO registrada: %+v", nuevaIo))
	w.WriteHeader(http.StatusOK)
}
