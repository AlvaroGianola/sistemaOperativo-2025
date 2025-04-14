package ioUtils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	ioGlobalUtils "github.com/sisoputnfrba/tp-golang/io/globalsIO"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

// Lee el archivo de configuración y lo parsea en la estructura Config
func IniciarConfiguracion(filePath string) *ioGlobalUtils.Config {
	var config *ioGlobalUtils.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}


func RecibirPeticion(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error leyendo el body de la petición: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

defer r.Body.Close()

	type RequestIO struct {
		PID    int `json:"pid"`
		Tiempo int `json:"tiempo"`
	}

	var req RequestIO
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("Error parseando JSON: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Log obligatorio de inicio de IO
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Inicio de IO - Tiempo: %d", req.PID, req.Tiempo))

	// Simula la ejecución del IO (usleep equivalente con time.Sleep)
	time.Sleep(time.Duration(req.Tiempo) * time.Millisecond)

	// Log obligatorio de fin de IO
	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Fin de IO", req.PID))

	w.WriteHeader(http.StatusOK)
}

// Envía un handshake al Kernel informando el nombre del IO, su IP local y puerto en el que se levanta
func EnviarHandshakeAKernel(nombre string) {
	ipIO, err := clientUtils.ObtenerIPLocal()
	if err != nil {
		log.Printf("Error al obtener ip de la Io")
		return
	}

	puertoIo := strconv.Itoa(ioGlobalUtils.IoConfig.PortIO)

	valores := []string{nombre, ipIO, puertoIo}

	// El último parámetro "ios" representa el tipo de paquete
	clientUtils.GenerarYEnviarPaquete(valores, ioGlobalUtils.IoConfig.IPKernel, ioGlobalUtils.IoConfig.PuertoKernel, "ios") //IP y Puerto de la CPU

}

