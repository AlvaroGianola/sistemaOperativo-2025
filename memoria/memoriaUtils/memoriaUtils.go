package memoriaUtils

import (
	"encoding/json"
	"strconv"
	"strings"

	//"errors"
	"net/http"
	"os"

	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

var procesos map[int]globalsMemoria.Proceso = make(map[int]globalsMemoria.Proceso)

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

const (
	PID = iota
	FILE_PATH
	SIZE
	MOVIMIENTO_EN_TABLA
)

func IniciarProceso(w http.ResponseWriter, r *http.Request) {

	clientUtils.Logger.Info("[Memoria] Petición para inicar proceso recibida desde Kernel")

	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[PID])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(pedido.Valores[SIZE])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear el tamaño del proceso")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	//chequeo si el pid ya existe
	globalsMemoria.MutexProcesos.Lock()
	defer globalsMemoria.MutexProcesos.Unlock()

	if buscarProceso(pid) != nil {
		clientUtils.Logger.Error("Proceso con Pid ya existe:", "pid especifico", pid)
		http.Error(w, "PID ya existe", http.StatusConflict)
		return
	}

	//aca va a leer el path
	instrucciones, err := os.ReadFile(globalsMemoria.MemoriaConfig.ScriptsPath + pedido.Valores[FILE_PATH])
	if err != nil {
		clientUtils.Logger.Error("Error al leer el path:", "error", err)
		http.Error(w, "Path invalido", http.StatusBadRequest)
		return
	} //esto tamien contempla problemas con el path

	if EspacioLibre() < size {
		http.Error(w, "Espacio en memoria insuficiete.", http.StatusBadRequest)
		return
	}

	errInterno := asignarMemoria(pid, instrucciones)
	if errInterno != false {
		clientUtils.Logger.Error("Error al asignar memoria:", "error", errInterno)
		http.Error(w, "Error al asignar memoria", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	clientUtils.Logger.Info("Se crea el proceso", "PID", pid, "Tamaño", pedido.Valores[SIZE])
}

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para finalizar proceso recibida desde Kernel")

	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[PID])
	if err != nil {
		clientUtils.Logger.Error("Error decodificando el body:", "error", err)
		http.Error(w, "Body inválido", http.StatusBadRequest)
		return
	}
	globalsMemoria.MutexProcesos.Lock()
	defer globalsMemoria.MutexProcesos.Unlock()

	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}
	// Eliminar el proceso de la lista
	delete(procesos, pid)

	clientUtils.Logger.Info("Se finaliza el proceso", "PID", pid, "Tamaño", proceso.Size)
	clientUtils.Logger.Info("Espacio libre en memoria:", "espacio", EspacioLibre())

	w.WriteHeader(http.StatusOK)
}

// Va a tener que recibir un PID y un PC (en ese orden) y responder con la siguiente instruccion
const (
	PC = 1
)

func SiguienteInstruccion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para inicar proceso recibida desde Kernel")

	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[PID])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	pc, err := strconv.Atoi(pedido.Valores[PC])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PC")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	globalsMemoria.MutexProcesos.Lock()
	defer globalsMemoria.MutexProcesos.Unlock()

	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	if pc < 0 || pc >= len(globalsMemoria.MemoriaUsuario) || pc >= proceso.Size {
		clientUtils.Logger.Error("PC fuera de rango:", "pc", pc)
		http.Error(w, "PC fuera de rango", http.StatusBadRequest)
		return
	}
	instruccion := globalsMemoria.MemoriaUsuario[pc]

	clientUtils.Logger.Info("Instrucción siguiente:", "pid", pid, "pc", pc, "instrucción", instruccion)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte{instruccion})
}

func EspacioLibre() int {
	var marcosLibres int = countMarcosLibres()
	var espacioLibre int = marcosLibres * globalsMemoria.MemoriaConfig.PageSize
	return espacioLibre
}

/*
func AccederTablaPaginasGlobal(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para acceder a la tabla de páginas recibida desde CPU")
	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[PID])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	movimientoEnTabla, err := strconv.Atoi(pedido.Valores[MOVIMIENTO_EN_TABLA])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear movimiento en tabla")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso := buscarProceso(pid)

	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}
	if movimientoEnTabla < 0 || movimientoEnTabla >= proceso.Size {
		clientUtils.Logger.Error("Dirección lógica fuera de rango:", movimientoEnTabla)
		http.Error(w, "Dirección lógica fuera de rango", http.StatusBadRequest)
		return
	}

	// Acceso a la tabla de páginas

	direccionFisica := proceso.TablaPaginasGlobal[movimientoEnTabla]

	proceso.Metricas.AccesosATablas++

	clientUtils.Logger.Info("Tabla de páginas global accedida", "pid", pid)

	w.Write(direccionFisica)
	w.WriteHeader(http.StatusOK)
}*/

func AccederPagina(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para acceder a una página recibida desde CPU")
	var datos struct {
		DireccionFisicaBase int `json:"direccion_fisica"`
		Offset              int `json:"offset"`
	}

	err := json.NewDecoder(r.Body).Decode(&datos)
	if err != nil {
		clientUtils.Logger.Error("Error decodificando el body:", "error", err)
		http.Error(w, "Body inválido", http.StatusBadRequest)
		return
	}

	if datos.DireccionFisicaBase < 0 || datos.Offset < 0 || datos.Offset >= globalsMemoria.MemoriaConfig.PageSize {
		clientUtils.Logger.Error("Dirección física o offset fuera de rango", "direccion_fisica", datos.DireccionFisicaBase, "offset", datos.Offset)
		http.Error(w, "Dirección física o offset fuera de rango", http.StatusBadRequest)
		return
	}

	// Acceso a la memoria
	direccionFisica := datos.DireccionFisicaBase + datos.Offset

	contenidoDireccionFisica := globalsMemoria.MemoriaUsuario[direccionFisica]

	clientUtils.Logger.Info("Página accedida. Pagina:", datos.DireccionFisicaBase, " Offset:", datos.Offset)
	w.Write([]byte{contenidoDireccionFisica})
	w.WriteHeader(http.StatusOK)
}

func AccederMarcoUsuario(w http.ResponseWriter, r *http.Request) {

}

func LeerPagina(w http.ResponseWriter, r *http.Request) {

}

func EscribirPagina(w http.ResponseWriter) {

}

func ObtenerConfiguracionMemoria(w http.ResponseWriter, r *http.Request) {
	//esto es lo que pide cpu para saber tamaño de pagina, cantidad de niveles, etc que esta en mi config
	tamanioPagina := globalsMemoria.MemoriaConfig.PageSize
	niveles := globalsMemoria.MemoriaConfig.NumberOfLevels
	entradas := globalsMemoria.MemoriaConfig.EntriesPerPage

	w.Write([]byte{})
	w.WriteHeader(http.StatusOK)

}

func SuspenderProceso(w http.ResponseWriter, r *http.Request) {
	//a implementar
}
func DesuspenderProceso(w http.ResponseWriter, r *http.Request) {
	//a implementar
}

func DumpMemoria(w http.ResponseWriter, r *http.Request) {

}

/////////////----------FUNC AUXILIARES----------/////////////

func ParsearInstrucciones(archivo []byte) []string {
	todasLasInstrucciones := string(archivo)
	lineas := strings.Split(todasLasInstrucciones, "\n")
	var instrucciones []string
	for _, linea := range lineas {
		instruccion := strings.TrimSpace(linea)
		if instruccion != "" {
			instrucciones = append(instrucciones, instruccion)
		}
	}
	return instrucciones
}

func buscarProceso(pid int) *globalsMemoria.Proceso {
	globalsMemoria.MutexProcesos.Lock()
	defer globalsMemoria.MutexProcesos.Unlock()

	procesos := globalsMemoria.ProcesosEnMemoria
	for i := range procesos {
		if procesos[i].Pid == pid {
			globalsMemoria.MutexProcesos.Unlock()
			return &procesos[i]
		}
	}

	globalsMemoria.MutexProcesos.Unlock()
	return nil
}

func countMarcosLibres() int {
	globalsMemoria.MutexBitmapMarcosLibres.Lock()
	defer globalsMemoria.MutexBitmapMarcosLibres.Unlock()

	count := 0
	for _, value := range globalsMemoria.BitmapMarcosLibres {
		if value {
			count++
		}
	}
	return count
}

func asignarMemoria(pid int, instrucciones []byte) bool {
	// Datos de configuración
	pageSize := globalsMemoria.MemoriaConfig.PageSize
	entriesPerPage := globalsMemoria.MemoriaConfig.EntriesPerPage
	numLevels := globalsMemoria.MemoriaConfig.NumberOfLevels

	// Crear proceso y su tabla global (nivel 1)
	proceso := &globalsMemoria.ProcesosEnMemoria[pid]
	proceso.Pid = pid
	proceso.Size = len(instrucciones)
	proceso.TablaPaginasGlobal = globalsMemoria.NewTablaPaginas(1)

	// Fragmentar instrucciones en páginas
	totalPaginas := len(instrucciones) / pageSize

	for i := 0; i < totalPaginas; i++ {
		// Buscar marco libre y crear página
		marco := buscarMarcoLibre()
		if marco == -1 {
			clientUtils.Logger.Error("No hay marcos libres disponibles para asignar memoria")
			return false // No hay marcos libres
		}
		globalsMemoria.BitmapMarcosLibres[marco] = false

		pagina := globalsMemoria.NewPagina(marco, true, true, true)

		// Cargar instrucciones en MemoriaUsuario
		inicio := i * pageSize
		fin := min((i+1)*pageSize, len(instrucciones))
		for j := inicio; j < fin; j++ {
			direccionFisica := marco*pageSize + (j - inicio)
			globalsMemoria.MemoriaUsuario[direccionFisica] = instrucciones[j]
		}

		// Insertar página en la jerarquía de tablas multinivel
		insertarPaginaEnJerarquia(&proceso.TablaPaginasGlobal, &pagina, i, numLevels, entriesPerPage)
	}
	return true
}

func insertarPaginaEnJerarquia(tabla *globalsMemoria.TablaPaginas, pagina *globalsMemoria.Pagina, nroPagina int, niveles int, entradasPorNivel int) {
	// Navegar o crear jerarquía desde Nivel 1 hasta Nivel N-1
	actual := tabla
	for nivel := 1; nivel < niveles; nivel++ {
		indice := calcularIndice(nroPagina, nivel, niveles, entradasPorNivel)
		siguiente := actual.Entradas[indice]
		if siguiente == nil {
			nuevaTabla := globalsMemoria.NewTablaPaginas(nivel + 1)
			actual.Entradas[indice] = &nuevaTabla
			actual = &nuevaTabla
		} else {
			tablaExistente, esTabla := siguiente.(*globalsMemoria.TablaPaginas)
			if !esTabla {
				// error: la entrada no debería ser una página acá
			}
			actual = tablaExistente
		}
	}

	// Nivel N: insertar página real
	indiceFinal := calcularIndice(nroPagina, niveles, niveles, entradasPorNivel)
	actual.Entradas[indiceFinal] = pagina
}

func calcularIndice(nroPagina, nivelActual, niveles, entradasPorNivel int) int {
	divisor := 1
	for i := 0; i < niveles-nivelActual; i++ {
		divisor *= entradasPorNivel
	}
	return (nroPagina / divisor) % entradasPorNivel
}

func buscarMarcoLibre() int {
	globalsMemoria.MutexBitmapMarcosLibres.Lock()
	defer globalsMemoria.MutexBitmapMarcosLibres.Unlock()
	if countMarcosLibres() > 0 {
		for i, libre := range globalsMemoria.BitmapMarcosLibres {
			if libre {
				return i
			}
		}
	}
	// No hay marcos libres
	return -1
}
