package memoriaUtils

import (
	"encoding/json"
	"net/http"
	"os"

	globalsmemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

// Inicia la configuración leyendo el archivo JSON correspondiente
func IniciarConfiguracion(filePath string) *globalsmemoria.Config {
	var config *globalsmemoria.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

// RecibirPeticionCpu es el endpoint para recibir mensajes de CPU
// Por ahora solo responde 200 OK y loguea la llegada
func RecibirPeticionCpu(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición recibida desde CPU")
	w.WriteHeader(http.StatusOK)
}

// RecibirPeticionKernel es el endpoint para recibir mensajes del Kernel
// Por ahora solo responde 200 OK y loguea la llegada
func RecibirPeticionKernel(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición recibida desde Kernel")
	w.WriteHeader(http.StatusOK)
}

// Por ahora solo responde 200 OK y loguea la llegada
// Va a tener que recibir un PID y un Path del archivo de pseudocodigo (en ese orden)
func InciarProceso(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para inicar proceso recibida desde Kernel")
	w.WriteHeader(http.StatusOK)
}

// Por ahora solo responde 200 OK y loguea la llegada
func FinalizarProceso(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para finalizar proceso recibida desde Kernel")
	w.WriteHeader(http.StatusOK)
}

// Por ahora solo responde 200 OK y loguea la llegada
// Va a tener que recibir un PID y un PC (en ese orden) y responder con la siguiente instruccion
func SiguienteInstruccion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para inicar proceso recibida desde Kernel")
	w.WriteHeader(http.StatusOK)
}
