package ioUtils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	ioGlobalUtils "github.com/sisoputnfrba/tp-golang/io/globalsIO"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)
var Nombre string
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

const(
	PID = iota
	TIME
)

func RecibirPeticion(w http.ResponseWriter, r *http.Request) {

	peticion := serverUtils.RecibirPaquetes(w,r)

	
	// Log obligatorio de inicio de IO
	clientUtils.Logger.Info(fmt.Sprintf("PID: %s - Inicio de IO - Tiempo: %s", peticion.Valores[PID], peticion.Valores[TIME]))

	// Simula la ejecución del IO (usleep equivalente con time.Sleep)
    milisegundos, err := strconv.Atoi(peticion.Valores[TIME])
    if err != nil {
		clientUtils.Logger.Error("Error al convertir el tiempo")
        return
    }

    // Convertir a time.Duration y dormir
    time.Sleep(time.Duration(milisegundos) * time.Millisecond)
	// Log obligatorio de fin de IO
	clientUtils.Logger.Info(fmt.Sprintf("PID: %s - Fin de IO", peticion.Valores[PID]))
	avisarFinIO(peticion.Valores[PID])

	w.WriteHeader(http.StatusOK)
}

// Envía un handshake al Kernel informando el nombre del IO, su IP local y puerto en el que se levanta
func EnviarHandshakeAKernel(nombre string, puertoIo int) {

	valores := []string{nombre, ioGlobalUtils.IoConfig.IPIo, strconv.Itoa(puertoIo)}

	// El último parámetro "ios" representa el end point
	clientUtils.GenerarYEnviarPaquete(valores, ioGlobalUtils.IoConfig.IPKernel, ioGlobalUtils.IoConfig.PortKernel, "ios") //IP y Puerto de la CPU

}

func avisarFinIO(PID string){
	valores := []string{Nombre,"Fin", PID}
	endpoint := "resultadoIos"
	clientUtils.GenerarYEnviarPaquete(valores, ioGlobalUtils.IoConfig.IPKernel, ioGlobalUtils.IoConfig.PortKernel, endpoint)
}

func AvisarDesconexion() {
	valores := []string{Nombre,"Desconexion"}
	endpoint := "resultadosIos" 
	clientUtils.GenerarYEnviarPaquete(valores, ioGlobalUtils.IoConfig.IPKernel, ioGlobalUtils.IoConfig.PortKernel, endpoint)
	clientUtils.Logger.Info(fmt.Sprintf("[IO] Dispositivo %s notifica su cierre al Kernel", Nombre))
}