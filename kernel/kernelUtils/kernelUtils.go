package kernelUtils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	globalsKernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
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
	Indentificador string `json:"identificador"`
	Ip             string `json:"ip"`
	Puerto         int    `json:"puerto"`
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

	puerto, err := strconv.Atoi(paquete.Valores[2])
	if err != nil {
		clientUtils.Logger.Info("Error al parsear puerto de CPU")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	nuevaCpu := Cpu{
		Indentificador: paquete.Valores[0],
		Ip:             paquete.Valores[1],
		Puerto:         puerto,
	}

	cpusRegistradas = append(cpusRegistradas, nuevaCpu)
	clientUtils.Logger.Info(fmt.Sprintf("CPU registrada: %+v", nuevaCpu))
}

// ResultadoProcesos es un endpoint placeholder para futuras devoluciones de la CPU
func ResultadoProcesos(w http.ResponseWriter, r *http.Request) {
	resultados := serverUtils.RecibirPaquetes(w, r)

	if len(resultados.Valores) == 0 {
		clientUtils.Logger.Warn("Resultado vacío")
		http.Error(w, "Resultado sin datos", http.StatusBadRequest)
		return
	}

	clientUtils.Logger.Info(fmt.Sprintf("Recibido resultado: %v", resultados.Valores))
	w.WriteHeader(http.StatusOK)

}

// RegistrarIo maneja el handshake de una IO
// Espera recibir un JSON con formato ["nombre", "ip", "puerto"]
func RegistrarIo(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)

	puerto, err := strconv.Atoi(paquete.Valores[2])
	if err != nil {
		clientUtils.Logger.Info("Error al parsear puerto de IO")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	nuevaIo := Io{
		Nombre: paquete.Valores[0],
		Ip:     paquete.Valores[1],
		Puerto: puerto,
	}

	iosRegistradas = append(iosRegistradas, nuevaIo)
	clientUtils.Logger.Info(fmt.Sprintf("IO registrada: %+v", nuevaIo))
}
