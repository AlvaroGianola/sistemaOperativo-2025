package memoriaUtils

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	//"errors"
	"net/http"
	"os"

	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

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

// Por ahora solo responde 200 OK y loguea la llegada
// Va a tener que recibir un PID y un PC (en ese orden) y responder con la siguiente instruccion

func Swapear() error {
	return nil
}

// Inicia la configuración leyendo el archivo JSON correspondiente

const (
	PID = iota
	FILE_PATH
	SIZE
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

	if buscarProceso(pid) != nil {
		clientUtils.Logger.Error("Proceso con Pid ya existe:", "pid especifico", pid)
		http.Error(w, "PID ya existe", http.StatusConflict)
		return
	}

	clientUtils.Logger.Info("LLEGUE BUSCAR")
	////////////////////////////////////////////////////////////

	//esto va a leer el path
	instruccionesSinParsear, err := os.ReadFile(globalsMemoria.MemoriaConfig.ScriptsPath + pedido.Valores[FILE_PATH])
	if err != nil {
		clientUtils.Logger.Error("Error al leer el path:", "error", err)
		http.Error(w, "Path invalido", http.StatusBadRequest)
		return
	} //esto tamien contempla problemas con el path

	listaInstrucciones := ParsearInstrucciones(instruccionesSinParsear)

	clientUtils.Logger.Info("LLEGUE INSTRUCCIONES")

	if EspacioLibre() < size {
		http.Error(w, "Espacio en memoria insuficiete.", http.StatusBadRequest)
		return
	}

	clientUtils.Logger.Info("LLEGUE ESPACIO")

	errInterno := !asignarMemoria(pid, listaInstrucciones)
	if errInterno {
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

	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	// Liberar los marcos de memoria asignados al proceso seteando el bitmap a true
	liberarTabla(&proceso.TablaPaginasGlobal, 1)

	// Eliminar el proceso del slice ProcesosEnMemoria
	for i := range globalsMemoria.ProcesosEnMemoria {
		if globalsMemoria.ProcesosEnMemoria[i].Pid == pid {
			globalsMemoria.ProcesosEnMemoria = append(globalsMemoria.ProcesosEnMemoria[:i], globalsMemoria.ProcesosEnMemoria[i+1:]...)
			break
		}
	}

	clientUtils.Logger.Info("Se finaliza el proceso", "PID", pid, "Tamaño", proceso.Size)
	clientUtils.Logger.Info("Espacio libre en memoria:", "espacio", EspacioLibre())

	w.WriteHeader(http.StatusOK)
}

// Va a tener que recibir un PID y un PC (en ese orden) y responder con la siguiente instruccion
const (
	PC = 1
)

func SiguienteInstruccion(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para siguiente instuccion recibida desde CPU")

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

	proceso := buscarProceso(pid)

	if pc < 0 || pc >= len(proceso.Instrucciones) {
		clientUtils.Logger.Error("PC fuera de rango:", "pc", pc)
		http.Error(w, "PC fuera de rango", http.StatusBadRequest)
		return
	}

	instruccion := proceso.Instrucciones[pc]

	clientUtils.Logger.Info("Instrucción siguiente:", "pid", pid, "pc", pc, "instrucción", instruccion)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(instruccion))
}

func AccederMarcoUsuario(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para acceder a un marco de usuario recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parsear todos los movimientos
	var movimientos []int
	for i := 1; i < len(pedido.Valores); i++ {
		valor, err := strconv.Atoi(pedido.Valores[i])
		if err != nil {
			clientUtils.Logger.Error("Error al parsear movimientos")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		movimientos = append(movimientos, valor)
	}

	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	// Acceder recursivamente a las tablas
	actual := &proceso.TablaPaginasGlobal // tabla raíz

	for nivel := 0; nivel < len(movimientos)-1; nivel++ {
		mov := movimientos[nivel]
		tabla, ok := actual.Entradas[mov].(*globalsMemoria.TablaPaginas)
		if !ok {
			clientUtils.Logger.Error("Error: se esperaba tabla en nivel", "nivel", nivel)
			http.Error(w, "Estructura incorrecta", http.StatusInternalServerError)
			return
		}
		proceso.Metricas.AccesosATablas++
		actual = tabla
	}

	// Último nivel: acceder a la página
	ultimoMovimiento := movimientos[len(movimientos)-1]
	pagina, ok := actual.Entradas[ultimoMovimiento].(*globalsMemoria.Pagina)
	if !ok {
		clientUtils.Logger.Error("Error: se esperaba página en último nivel")
		http.Error(w, "No se encontró la página", http.StatusInternalServerError)
		return
	}

	direccionFisica := pagina.Marco

	clientUtils.Logger.Info("Marco de usuario accedido", "pid", pid, "marco", direccionFisica)

	time.Sleep(time.Duration(globalsMemoria.MemoriaConfig.MemoryDelay) * time.Millisecond)
	w.Write([]byte(strconv.Itoa(direccionFisica)))
	w.WriteHeader(http.StatusOK)

}

func LeerPagina(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para leer una página recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if buscarProceso(pid) == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	marco, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear marco")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	tamanioEnviado, err := strconv.Atoi(pedido.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear tamaño enviado")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if tamanioEnviado != globalsMemoria.MemoriaConfig.PageSize {
		clientUtils.Logger.Error("Tamaño de página no coincide con el configurado")
		http.Error(w, "Tamaño de página incorrecto", http.StatusBadRequest)
		return
	}
	contenido := []byte{}
	// ver de implementar semaforos para no leer cosas mientras otro la modifica
	for i := 0; i < globalsMemoria.MemoriaConfig.PageSize; i++ {
		if marco+i >= len(globalsMemoria.MemoriaUsuario) {
			clientUtils.Logger.Error("Acceso fuera de rango a la memoria", "marco", marco, "offset", i)
			http.Error(w, "Acceso fuera de rango a la memoria", http.StatusBadRequest)
			return
		}
		if globalsMemoria.BitmapMarcosLibres[marco+i] {
			clientUtils.Logger.Error("Marco no asignado a ningún proceso", "marco", marco+i)
			http.Error(w, "Marco no asignado a ningún proceso", http.StatusBadRequest)
			return
		}
		contenido = append(contenido, globalsMemoria.MemoriaUsuario[marco+i])
	}

	// Simulamos la lectura de la página, en realidad deberíamos leer desde el marco

	clientUtils.Logger.Info("Página leída", "pid", pid, "marco", marco, "contenido", contenido)
	w.Write([]byte(contenido))
	w.WriteHeader(http.StatusOK)
}

func EscribirPagina(w http.ResponseWriter, r *http.Request) {

	clientUtils.Logger.Info("[Memoria] Petición para escribir una página recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if buscarProceso(pid) == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}
	marco, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear marco")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	tamanioEnviado, err := strconv.Atoi(pedido.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear tamaño enviado")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if tamanioEnviado != globalsMemoria.MemoriaConfig.PageSize {
		clientUtils.Logger.Error("Tamaño de página no coincide con el configurado")
		http.Error(w, "Tamaño de página incorrecto", http.StatusBadRequest)
		return
	}

	for i := 0; i < globalsMemoria.MemoriaConfig.PageSize; i++ {
		if marco+i >= len(globalsMemoria.MemoriaUsuario) {
			clientUtils.Logger.Error("Acceso fuera de rango a la memoria", "marco", marco, "offset", i)
			http.Error(w, "Acceso fuera de rango a la memoria", http.StatusBadRequest)
			return
		}
		contenido, err := strconv.Atoi(pedido.Valores[3+i])
		if err != nil {
			clientUtils.Logger.Error("Error al parsear contenido")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		globalsMemoria.MemoriaUsuario[marco+i] = byte(contenido)
	}

	clientUtils.Logger.Info("Página escrita", "pid", pid, "marco", marco, "tamaño", tamanioEnviado)
	w.WriteHeader(http.StatusOK)

}

func LeerDireccionFisica(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para leer dirección física recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if buscarProceso(pid) == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	direccionFisica, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear dirección física")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	contenido := globalsMemoria.MemoriaUsuario[direccionFisica]
	// Simulamos la escritura de la dirección física

	w.Write([]byte{contenido})
	w.WriteHeader(http.StatusOK)
}

func EscribirDireccionFisica(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para escribir dirección física recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if buscarProceso(pid) == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	direccionFisica, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear dirección física")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	contenido, err := strconv.Atoi(pedido.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear contenido")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	globalsMemoria.MemoriaUsuario[direccionFisica] = byte(contenido)

	w.WriteHeader(http.StatusOK)

}

func ObtenerConfiguracionMemoria(w http.ResponseWriter, r *http.Request) {
	//esto es lo que pide cpu para saber tamaño de pagina, cantidad de niveles, etc que esta en mi config

	//hacer un json que tenga los tres datos y enviarlo
	clientUtils.Logger.Info("[Memoria] Petición para obtener configuración de memoria recibida desde CPU")
	configuracion := struct {
		TamanioPagina    int `json:"tamanioPagina"`
		Niveles          int `json:"niveles"`
		EntradasPorNivel int `json:"entradasPorNivel"`
	}{
		TamanioPagina:    globalsMemoria.MemoriaConfig.PageSize,
		Niveles:          globalsMemoria.MemoriaConfig.NumberOfLevels,
		EntradasPorNivel: globalsMemoria.MemoriaConfig.EntriesPerPage,
	}
	// Enviar la configuración como un JSON
	configuracionJSON, err := json.Marshal(configuracion)
	if err != nil {
		clientUtils.Logger.Error("Error al codificar la configuración de memoria:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(configuracionJSON))

}

func SuspenderProceso(w http.ResponseWriter, r *http.Request) {
	//a implementar
}
func DesuspenderProceso(w http.ResponseWriter, r *http.Request) {
	//a implementar
}

func DumpMemoria(w http.ResponseWriter, r *http.Request) {
	//a implementar
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////----------FUNC AUXILIARES----------////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
			return &procesos[i]
		}
	}

	return nil
}

func countMarcosLibres() int {
	clientUtils.Logger.Info("Contando marcos libres")
	globalsMemoria.MutexContadorMarcosLibres.Lock()
	defer globalsMemoria.MutexContadorMarcosLibres.Unlock()

	count := 0
	for _, value := range globalsMemoria.BitmapMarcosLibres {
		if value {
			count++
		}
	}
	return count
}

func asignarMemoria(pid int, instrucciones []string) bool {
	clientUtils.Logger.Info("Asignando memoria al proceso", "pid", pid, "tamaño", len(instrucciones))
	pageSize := globalsMemoria.MemoriaConfig.PageSize
	numLevels := globalsMemoria.MemoriaConfig.NumberOfLevels

	if pid < 0 {
		clientUtils.Logger.Error("PID negativo no permitido")
		return false
	}

	// Si hay concurrencia en ProcesosEnMemoria, deberías protegerlo con mutex.
	// globalsMemoria.MutexProcesos.Lock()
	// defer globalsMemoria.MutexProcesos.Unlock()

	if pid >= len(globalsMemoria.ProcesosEnMemoria) {
		nuevo := make([]globalsMemoria.Proceso, pid-len(globalsMemoria.ProcesosEnMemoria)+1)
		globalsMemoria.ProcesosEnMemoria = append(globalsMemoria.ProcesosEnMemoria, nuevo...)
	}

	proceso := &globalsMemoria.ProcesosEnMemoria[pid]
	proceso.Pid = pid
	proceso.Size = len(instrucciones)
	proceso.Instrucciones = instrucciones
	proceso.TablaPaginasGlobal = globalsMemoria.NewTablaPaginas(1)

	totalPaginas := (len(instrucciones) + pageSize - 1) / pageSize
	marcosAsignados := []int{}

	for i := 0; i < totalPaginas; i++ {
		marco := buscarMarcoLibre()
		if marco == -1 {
			clientUtils.Logger.Error("No hay marcos libres disponibles para asignar memoria")
			rollback(proceso, marcosAsignados)
			return false
		}
		marcosAsignados = append(marcosAsignados, marco)
		pagina := globalsMemoria.NewPagina(marco, true, true, true)

		inicio := i * pageSize
		fin := min((i+1)*pageSize, len(instrucciones))

		for j := inicio; j < fin; j++ {
			direccionFisica := marco*pageSize + (j - inicio)
			if direccionFisica < 0 || direccionFisica >= len(globalsMemoria.MemoriaUsuario) {
				clientUtils.Logger.Error("Acceso fuera de rango a MemoriaUsuario", "direccionFisica", direccionFisica)
				rollback(proceso, marcosAsignados)
				return false
			}
			globalsMemoria.MemoriaUsuario[direccionFisica] = 1
		}

		if err := insertarPaginaEnJerarquia(&proceso.TablaPaginasGlobal, &pagina, i, numLevels); err {
			clientUtils.Logger.Error("Error al insertar página en jerarquía", "error", err)
			rollback(proceso, marcosAsignados)
			return false
		}
	}

	return true
}
func rollback(proceso *globalsMemoria.Proceso, marcos []int) {
	globalsMemoria.MutexBitmapMarcosLibres.Lock()
	for _, m := range marcos {
		globalsMemoria.BitmapMarcosLibres[m] = true
	}
	globalsMemoria.MutexBitmapMarcosLibres.Unlock()

	liberarTabla(&proceso.TablaPaginasGlobal, 1)
}

func insertarPaginaEnJerarquia(tabla *globalsMemoria.TablaPaginas, pagina *globalsMemoria.Pagina, nroPagina int, niveles int) bool {
	// Navegar o crear jerarquía desde Nivel 1 hasta Nivel N-1
	actual := tabla
	for nivel := 1; nivel < niveles; nivel++ {
		indice := calcularIndice(nroPagina, nivel)
		siguiente := actual.Entradas[indice]
		if siguiente == nil {
			nuevaTabla := globalsMemoria.NewTablaPaginas(nivel + 1)
			actual.Entradas[indice] = &nuevaTabla
			actual = &nuevaTabla
		} else {
			tablaExistente, esTabla := siguiente.(*globalsMemoria.TablaPaginas)
			if !esTabla {
				// error: la entrada no debería ser una página acá
				return true
			}
			actual = tablaExistente
		}
	}

	// Nivel N: insertar página real
	indiceFinal := calcularIndice(nroPagina, niveles)
	actual.Entradas[indiceFinal] = pagina
	return false // No hubo error
}

func calcularIndice(nroPagina, nivelActual int) int {
	divisor := 1
	for i := 0; i < globalsMemoria.MemoriaConfig.NumberOfLevels-nivelActual; i++ {
		divisor *= globalsMemoria.MemoriaConfig.EntriesPerPage
	}
	return (nroPagina / divisor) % globalsMemoria.MemoriaConfig.NumberOfLevels
}

func buscarMarcoLibre() int {
	clientUtils.Logger.Info("Buscando marco libre")
	globalsMemoria.MutexBitmapMarcosLibres.Lock()
	defer globalsMemoria.MutexBitmapMarcosLibres.Unlock()
	clientUtils.Logger.Info("NoquedoEn dead lock con mutexBitmapMarcosLibres")
	if countMarcosLibres() > 0 {
		for i, libre := range globalsMemoria.BitmapMarcosLibres {
			if libre {
				globalsMemoria.BitmapMarcosLibres[i] = false
				return i
			}
		}
	}
	// No hay marcos libres
	return -1
}

func liberarTabla(tabla *globalsMemoria.TablaPaginas, nivelActual int) {
	for i, entrada := range tabla.Entradas {
		if entrada == nil {
			return
		}
		if nivelActual == globalsMemoria.MemoriaConfig.NumberOfLevels {
			// Es una página real
			pagina, ok := entrada.(*globalsMemoria.Pagina)
			if ok && pagina.Validez {
				globalsMemoria.MutexBitmapMarcosLibres.Lock()
				globalsMemoria.BitmapMarcosLibres[pagina.Marco] = true
				globalsMemoria.MutexBitmapMarcosLibres.Unlock()
			}
			tabla.Entradas[i] = nil
		} else {
			subtabla, ok := entrada.(*globalsMemoria.TablaPaginas)
			if ok {
				liberarTabla(subtabla, nivelActual+1)
			}
			// eliminar todo lo que esta apuntado por esa entrada como estamos en go no tengo que preocuparme por liberar memoria :)
			tabla.Entradas[i] = nil
		}
	}
}

func EspacioLibre() int {
	var marcosLibres int = countMarcosLibres()
	var espacioLibre int = marcosLibres * globalsMemoria.MemoriaConfig.PageSize
	return espacioLibre
}
