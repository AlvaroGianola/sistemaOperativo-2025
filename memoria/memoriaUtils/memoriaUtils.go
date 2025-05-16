package memoriaUtils

import (
	"encoding/json"
	"strings"
	"sync"

	//"errors"
	"net/http"
	"os"

	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

var mutexProcesos sync.Mutex

// Inicia la configuración leyendo el archivo JSON correspondiente

func IniciarConfiguracion(filePath string) *globalsMemoria.Config {

	config := &globalsMemoria.Config{} // Aca creamos el contenedor donde irá el JSON

	configFile, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(config)
	if err != nil {
		panic("Error al decodificar config: " + err.Error())
	}

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
func IniciarProceso(w http.ResponseWriter, r *http.Request) {
	//podria mejorar haciendo funciones auxiliares y cambiando el globalsMemoria.proceso

	clientUtils.Logger.Info("[Memoria] Petición para inicar proceso recibida desde Kernel")

	// decodifica los datos
	var datosInstruccion struct {
		Pid  int    `json:"pid"`
		Path string `json:"path"`
	}
	err := json.NewDecoder(r.Body).Decode(&datosInstruccion)
	if err != nil {
		clientUtils.Logger.Error("Error decodificando el body:", "error", err)
		http.Error(w, "Body inválido", http.StatusBadRequest)
		return
	}
	//hasta esto es decode teniendo en cuenta posible error
	mutexProcesos.Lock()
	defer mutexProcesos.Unlock()

	if ExisteProceso(datosInstruccion.Pid) {
		clientUtils.Logger.Error("Proceso con Pid ya existe:", "pid especifico", datosInstruccion.Pid)
		http.Error(w, "PID ya existe", http.StatusConflict)
		return
	}

	//nose hasta que punto es necesario que el mutex afecte al parseo, pero lo dejo por las dudas nose como cambiarlo
	//la otra opcion es que el parseo se haga antes de que se bloquee el mutex, pero entonces parsearia
	//archivos sin chequear si el pid ya existe

	//esto va a leer el path
	instruccionesSinParsear, err := os.ReadFile(datosInstruccion.Path)
	if err != nil {
		clientUtils.Logger.Error("Error al leer el path:", "error", err)
		http.Error(w, "Path invalido", http.StatusBadRequest)
		return
	} //esto tamien contempla problemas con el path

	listaInstrucciones := ParsearInstrucciones(instruccionesSinParsear)

	procesoNuevo := globalsMemoria.Proceso{
		Pid:           datosInstruccion.Pid,
		Instrucciones: listaInstrucciones,
		Pc:            0,
	}

	globalsMemoria.ProcesosEnMemoria = append(globalsMemoria.ProcesosEnMemoria, procesoNuevo)

	w.WriteHeader(http.StatusOK)

	clientUtils.Logger.Info("Se crea el proceso", "PID", procesoNuevo.Pid, "Tamaño", len(procesoNuevo.Instrucciones))
}
func ParsearInstrucciones(archivo []byte) []string {
	todasLasInstrucciones := string(archivo)
	instruccionesSeparadas := strings.Split(todasLasInstrucciones, "\n")
	return instruccionesSeparadas
}

func ExisteProceso(Pid int) bool {
	for _, proceso := range globalsMemoria.ProcesosEnMemoria {
		if proceso.Pid == Pid {
			return true
		}

	}
	return false
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
	var datos struct {
		Pid int `json:"pid"`
		Pc  int `json:"pc"`
	}
	err := json.NewDecoder(r.Body).Decode(&datos)
	if err != nil {
		clientUtils.Logger.Error("Error decodificando el body:", "error", err)
		http.Error(w, "Body inválido", http.StatusBadRequest)
		return
	}
	mutexProcesos.Lock()
	defer mutexProcesos.Unlock()

	proceso := buscarProceso(globalsMemoria.ProcesosEnMemoria, datos.Pid)

	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", datos.Pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	if datos.Pc < 0 || datos.Pc >= len(proceso.Instrucciones) {
		clientUtils.Logger.Error("PC fuera de rango:", "pc", datos.Pc)
		http.Error(w, "PC fuera de rango", http.StatusBadRequest)
		return
	}
	instruccion := proceso.Instrucciones[datos.Pc]
	proceso.Pc = datos.Pc + 1
	clientUtils.Logger.Info("Instrucción siguiente:", "pid", datos.Pid, "pc", datos.Pc, "instrucción", instruccion)

	w.Write([]byte(instruccion))
	w.WriteHeader(http.StatusOK)
}

func buscarProceso(procesos []globalsMemoria.Proceso, pid int) *globalsMemoria.Proceso {
	for _, proceso := range procesos {
		if proceso.Pid == pid {
			return &proceso
		}
	}
	return nil
}

func EspacioLibre() int {
	//en un futuro debera calcular y retornar el espacio libre
	//por ahora retorna un valor fijo (mock)
	return 2048
}

func Swapear(*http.Request) error { // nose bien si esto es asi siquiera veremos dijo el ciego
	return nil
}
