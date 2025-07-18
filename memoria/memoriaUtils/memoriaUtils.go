package memoriaUtils

import (
	"encoding/json"
	"reflect" //eliminar despues de probar que funciona
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

	clientUtils.Logger.Info("[Memoria] Configuración de memoria cargada correctamente", "config", config)

	// crea el archivo swapfile.bin si no existe
	//si existe igual lo tengo que truncar a 0
	if _, err := os.Stat(config.SwapfilePath); os.IsNotExist(err) {
		file, err := os.Create(config.SwapfilePath)
		if err != nil {
			panic("Error al crear swapfile: " + err.Error())
		}
		defer file.Close()
	}
	// Trunca el archivo a 0 bytes
	file, err := os.OpenFile(config.SwapfilePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic("Error al truncar swapfile: " + err.Error())
	}
	defer file.Close()

	// crea el directorio de dumps si no existe
	if _, err := os.Stat(config.DumpPath); os.IsNotExist(err) {
		err := os.MkdirAll(config.DumpPath, 0755)
		if err != nil {
			panic("Error al crear directorio de dumps: " + err.Error())
		}
	}

	globalsMemoria.MemoriaUsuario = make([]byte, config.MemorySize)
	globalsMemoria.BitmapMarcosLibres = make([]bool, config.MemorySize/config.PageSize)
	for i := range globalsMemoria.BitmapMarcosLibres {
		globalsMemoria.BitmapMarcosLibres[i] = true
	}

	globalsMemoria.ProcesosEnMemoria = make([]*globalsMemoria.Proceso, 0)

	return config
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

	////////////////////////////////////////////////////////////

	//esto va a leer el path
	instruccionesSinParsear, err := os.ReadFile(globalsMemoria.MemoriaConfig.ScriptsPath + pedido.Valores[FILE_PATH])
	if err != nil {
		clientUtils.Logger.Error("Error al leer el path:", "error", err)
		http.Error(w, "Path invalido", http.StatusBadRequest)
		return
	} //esto tamien contempla problemas con el path

	listaInstrucciones := ParsearInstrucciones(instruccionesSinParsear)

	if EspacioLibre() < size {
		http.Error(w, "Espacio en memoria insuficiete.", http.StatusBadRequest)
		return
	}

	errInterno := !asignarMemoria(pid, listaInstrucciones, size)
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
	if proceso == nil {
		clientUtils.Logger.Error("PID no existente en memoria:", "pid", pid)
		http.Error(w, "PID no existente en memoria", http.StatusBadRequest)
		return
	}
	clientUtils.Logger.Info("Buscando proceso", "pid", pid)
	clientUtils.Logger.Info("Proceso encontrado", "pid", pid, "instrucciones", len(proceso.Instrucciones))
	if pc < 0 || pc > len(proceso.Instrucciones)-1 {
		clientUtils.Logger.Error("PC fuera de rango:", "pc", pc)
		http.Error(w, "PC fuera de rango", http.StatusBadRequest)
		return
	}

	instruccion := proceso.Instrucciones[pc]

	clientUtils.Logger.Info("Instrucción siguiente:", "pid", pid, "pc", pc, "instrucción", instruccion)

	proceso.Metricas.InstruccionesSolicitadas++
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(instruccion))
}

func AccederMarcoUsuario(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para acceder a un marco de usuario recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)
	clientUtils.Logger.Debug("Los valores recibidos en accederMarcoUsuario", "valores: ", pedido.Valores)

	if len(pedido.Valores) < 3 {
		clientUtils.Logger.Error("Error: paquete con cantidad insuficiente de valores")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parsear PID
	pid, err := strconv.Atoi(pedido.Valores[0])
	clientUtils.Logger.Debug("PID recibido", "pid", pid)
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID", "valor", pedido.Valores[0])
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parsear entradas de tabla (ignorando el último valor que es el desplazamiento)
	var movimientos []int
	for i := 1; i < len(pedido.Valores)-1; i++ {
		valor, err := strconv.Atoi(pedido.Valores[i])
		if err != nil {
			clientUtils.Logger.Error("Error al parsear entrada de tabla", "indice", i, "valor", pedido.Valores[i])
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		clientUtils.Logger.Info("Entrada de tabla recibida", "nivel", i, "valor", valor)
		movimientos = append(movimientos, valor)
	}
	clientUtils.Logger.Debug("Movimientos: ", "movimientos", movimientos)

	// Parsear desplazamiento (último valor)
	desplazamientoStr := pedido.Valores[len(pedido.Valores)-1]
	desplazamiento, err := strconv.Atoi(desplazamientoStr)
	clientUtils.Logger.Debug("Desplazamiento: ", "desplazamiento", desplazamiento)
	if err != nil {
		clientUtils.Logger.Error("Error al parsear desplazamiento", "valor", desplazamientoStr)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	clientUtils.Logger.Debug("Desplazamiento recibido", "valor", desplazamiento)

	// Buscar proceso
	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado", "pid", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	actual := &proceso.TablaPaginasGlobal

	/*if globalsMemoria.MemoriaConfig.NumberOfLevels == 1 {
		// Solo un nivel: acceder directo
		indice := movimientos[0]
		pagina, ok := actual.Entradas[indice].(*globalsMemoria.Pagina)
		if !ok {
			clientUtils.Logger.Error("Error: se esperaba página en único nivel")
			http.Error(w, "No se encontró la página", http.StatusInternalServerError)
			return
		}
		direccionFisica := pagina.Marco
		clientUtils.Logger.Info("Marco de usuario accedido (nivel 1)", "pid", pid, "marco", direccionFisica)
		time.Sleep(time.Duration(globalsMemoria.MemoriaConfig.MemoryDelay) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(strconv.Itoa(direccionFisica)))
		return
	}*/

	// Acceder recursivamente a las tablas de páginas si hay mas de un nivel

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
		clientUtils.Logger.Error("Error: se esperaba página en último nivel pero se encontró", "tipo", reflect.TypeOf(actual.Entradas[ultimoMovimiento]), "nivel", len(movimientos)-1, "movimiento", ultimoMovimiento)
		http.Error(w, "No se encontró la página", http.StatusInternalServerError)
		return
	}

	direccionFisica := pagina.Marco
	clientUtils.Logger.Info("Marco de usuario accedido", "pid", pid, "marco", direccionFisica)

	time.Sleep(time.Duration(globalsMemoria.MemoriaConfig.MemoryDelay) * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(direccionFisica)))
}

func LeerPagina(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para leer una página recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 3 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso := buscarProceso(pid)
	if proceso == nil {
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	marco, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	tamanioEnviado, err := strconv.Atoi(pedido.Valores[2])
	if err != nil || tamanioEnviado != globalsMemoria.MemoriaConfig.PageSize {
		http.Error(w, "Tamaño de página incorrecto", http.StatusBadRequest)
		return
	}

	// Validación del marco
	if marco < 0 || marco >= len(globalsMemoria.BitmapMarcosLibres) {
		http.Error(w, "Marco fuera de rango", http.StatusBadRequest)
		return
	}
	if globalsMemoria.BitmapMarcosLibres[marco] {
		http.Error(w, "Marco no asignado", http.StatusBadRequest)
		return
	}

	pageSize := globalsMemoria.MemoriaConfig.PageSize
	inicio := marco * pageSize
	fin := inicio + pageSize
	if fin > len(globalsMemoria.MemoriaUsuario) {
		http.Error(w, "Acceso fuera de rango a memoria física", http.StatusBadRequest)
		return
	}

	contenido := make([]byte, pageSize)
	copy(contenido, globalsMemoria.MemoriaUsuario[inicio:fin])

	proceso.Metricas.LecturasDeMemoria++
	clientUtils.Logger.Info("Página leída", "pid", pid, "marco", marco)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contenido)
}

func EscribirPagina(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para escribir una página recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 3 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso := buscarProceso(pid)
	if proceso == nil {
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	marco, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	tamanioEnviado, err := strconv.Atoi(pedido.Valores[2])
	if err != nil || tamanioEnviado != globalsMemoria.MemoriaConfig.PageSize {
		http.Error(w, "Tamaño de página incorrecto", http.StatusBadRequest)
		return
	}

	pageSize := globalsMemoria.MemoriaConfig.PageSize

	// Validar cantidad total de datos
	if len(pedido.Valores) != 3+pageSize {
		http.Error(w, "Datos incompletos para la escritura", http.StatusBadRequest)
		return
	}

	// Validación del marco
	if marco < 0 || marco >= len(globalsMemoria.BitmapMarcosLibres) {
		http.Error(w, "Marco fuera de rango", http.StatusBadRequest)
		return
	}
	if globalsMemoria.BitmapMarcosLibres[marco] {
		http.Error(w, "Marco no asignado", http.StatusBadRequest)
		return
	}

	inicio := marco * pageSize
	fin := inicio + pageSize
	if fin > len(globalsMemoria.MemoriaUsuario) {
		http.Error(w, "Acceso fuera de rango a memoria física", http.StatusBadRequest)
		return
	}

	// Escribir datos en memoria
	for i := 0; i < pageSize; i++ {
		contenido, err := strconv.Atoi(pedido.Valores[3+i])
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		if contenido < 0 || contenido > 255 {
			http.Error(w, "Valor fuera de rango (0-255)", http.StatusBadRequest)
			return
		}
		globalsMemoria.MemoriaUsuario[inicio+i] = byte(contenido)
	}

	proceso.Metricas.EscriturasDeMemoria++
	clientUtils.Logger.Info("Página escrita", "pid", pid, "marco", marco)

	w.WriteHeader(http.StatusOK)
}

func LeerDireccionFisica(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para leer dirección física recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	clientUtils.Logger.Debug("Los valores recibidos en leerDireccionFisica", "valores: ", pedido.Valores)

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	proceso := buscarProceso(pid)
	if proceso == nil {
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
	proceso.Metricas.LecturasDeMemoria++
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{contenido})

}

func EscribirDireccionFisica(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para escribir dirección física recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	clientUtils.Logger.Debug("Los valores recibidos en escribirDireccionFisica", "valores: ", pedido.Valores)

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	proceso := buscarProceso(pid)
	if proceso == nil {
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
	// ✅ Esta parte es clave:
	valorNumerico, err := strconv.Atoi(pedido.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear valor a byte")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	contenido := byte(valorNumerico)

	proceso.Metricas.EscriturasDeMemoria++
	globalsMemoria.MemoriaUsuario[direccionFisica] = contenido

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
	clientUtils.Logger.Info("[Memoria] Petición para suspender proceso recibida desde Kernel")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 1 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso := buscarProceso(pid)
	if proceso == nil {
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	// Leer contenido real de las páginas del proceso
	paginas := leerPaginasDeTabla(&proceso.TablaPaginasGlobal, 1)

	swapFile, err := os.OpenFile(globalsMemoria.MemoriaConfig.SwapfilePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		clientUtils.Logger.Error("Error al abrir swapfile:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	defer swapFile.Close()

	// Posicionar puntero en el offset libre
	offset := globalsMemoria.SiguienteOffsetLibre
	if _, err := swapFile.Seek(offset, 0); err != nil {
		clientUtils.Logger.Error("Error al posicionar en swapfile:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	n, err := swapFile.Write(paginas)
	// Escribir las páginas en swap
	if err != nil {
		clientUtils.Logger.Error("Error al escribir en swapfile:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	// Actualizar tabla de swap (con lock)
	globalsMemoria.MutexTablaSwap.Lock()
	globalsMemoria.TablaSwap[pid] = globalsMemoria.ProcesoEnSwap{
		Pid:    pid,
		Offset: offset,
		Size:   n,
	}
	globalsMemoria.SiguienteOffsetLibre += int64(n)
	globalsMemoria.MutexTablaSwap.Unlock()

	// Liberar marcos (protegido por MutexMemoria)
	liberarTabla(&proceso.TablaPaginasGlobal, 1)

	proceso.Metricas.BajadasASwap++
	clientUtils.Logger.Info("Proceso suspendido", "pid", pid, "bytes_escritos", len(paginas), "offset", offset)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso suspendido exitosamente"))
}

func DesuspenderProceso(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para desuspender proceso recibida desde Kernel")

	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso := buscarProceso(pid)
	clientUtils.Logger.Debug("Proceso para desuspender encontrado", "pid", pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}
	globalsMemoria.MutexTablaSwap.Lock()
	entrada, ok := globalsMemoria.TablaSwap[pid]
	globalsMemoria.MutexTablaSwap.Unlock()

	if !ok {
		http.Error(w, "PID no encontrado en TablaSwap", http.StatusNotFound)
		return
	}

	swapFile, err := os.Open(globalsMemoria.MemoriaConfig.SwapfilePath)
	if err != nil {
		http.Error(w, "Error al abrir swapfile", http.StatusInternalServerError)
		return
	}
	defer swapFile.Close()

	contenido := make([]byte, entrada.Size)

	_, err = swapFile.ReadAt(contenido, entrada.Offset)
	if err != nil {
		clientUtils.Logger.Error("Error al leer swapfile:", "error", err)
		http.Error(w, "Error al leer swapfile", http.StatusInternalServerError)
		return
	}

	if !reAsignarMemoria(pid, contenido, proceso.Size) {
		http.Error(w, "Error al reasignar memoria", http.StatusInternalServerError)
		return
	}

	proceso.Metricas.SubidasAMemoria++

	clientUtils.Logger.Info("Proceso desuspendido exitosamente:", "pid", pid)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso desuspendido exitosamente"))

}
func reAsignarMemoria(pid int, contenidoTotal []byte, size int) bool {
	clientUtils.Logger.Info("Reasignando memoria al proceso", "pid", pid, "tamaño", len(contenidoTotal))

	if pid < 0 {
		return false
	}
	if len(contenidoTotal) > EspacioLibre() {
		clientUtils.Logger.Error("Espacio insuficiente para reasignar memoria al proceso", "pid", pid, "tamaño requerido", len(contenidoTotal))
		return false
	}
	// Si hay concurrencia en ProcesosEnMemoria, deberías protegerlo con mutex.
	clientUtils.Logger.Debug("Espacio libre disponible", "espacio", EspacioLibre())

	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)

		return false
	}
	clientUtils.Logger.Debug("Proceso encontrado", "pid", pid, "tamaño", proceso.Size)
	pageSize := globalsMemoria.MemoriaConfig.PageSize
	numLevels := globalsMemoria.MemoriaConfig.NumberOfLevels

	totalPaginas := (size + pageSize - 1) / pageSize
	// esto te da el numero de paginas que necesitas si solo dividis te da la cantidad completa de paginas, pero si no es exacto te da una pagina mas
	marcosAsignados := []int{}

	for i := 0; i < totalPaginas; i++ {
		marco := buscarMarcoLibre()
		if marco == -1 {
			clientUtils.Logger.Error("No hay marcos libres disponibles para reasignar memoria")
			rollback(proceso, marcosAsignados)
			return false
		}
		marcosAsignados = append(marcosAsignados, marco)
		pagina := globalsMemoria.NewPagina(marco, true, true, true)

		inicio := i * pageSize
		fin := min((i+1)*pageSize, size)

		// Validar límites
		direccionInicioFisica := marco * pageSize
		direccionFinFisica := direccionInicioFisica + (fin - inicio)
		if direccionFinFisica > len(globalsMemoria.MemoriaUsuario) {
			clientUtils.Logger.Error("Acceso fuera de rango a MemoriaUsuario")
			rollback(proceso, marcosAsignados)
			return false
		}

		// Copiar el contenido real desde contenidoTotal
		copy(globalsMemoria.MemoriaUsuario[direccionInicioFisica:direccionFinFisica], contenidoTotal[inicio:fin])

		if err := insertarPaginaEnJerarquia(&proceso.TablaPaginasGlobal, &pagina, i, numLevels); err {
			clientUtils.Logger.Error("Error al insertar página en jerarquía", "error", err)
			rollback(proceso, marcosAsignados)
			return false
		}
	}

	//  limpiar la entradas swap
	globalsMemoria.MutexTablaSwap.Lock()
	delete(globalsMemoria.TablaSwap, pid)
	clientUtils.Logger.Debug("Tabla de swap limpiada para el proceso", "pid", pid)
	globalsMemoria.MutexTablaSwap.Unlock()
	return true
}

func DumpMemoria(w http.ResponseWriter, r *http.Request) {
	//a implementar
	clientUtils.Logger.Info("[Memoria] Petición para hacer dump de memoria recibida desde Kernel")

	pedido := serverUtils.RecibirPaquetes(w, r)
	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	//el archivo debe llamarse pid-timestamp.dmp
	archivoDump, err := os.Create(globalsMemoria.MemoriaConfig.DumpPath + strconv.Itoa(pid) + "-" + strconv.FormatInt(time.Now().Unix(), 10) + ".dmp")
	if err != nil {
		clientUtils.Logger.Error("Error al crear el nombre del archivo de dump:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	defer archivoDump.Close()

	//reservar espacio para el archivo de dump
	err = archivoDump.Truncate(int64(proceso.Size))
	if err != nil {
		clientUtils.Logger.Error("Error al reservar espacio en el archivo de dump:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	globalsMemoria.MutexProcesos.Lock()
	defer globalsMemoria.MutexProcesos.Unlock()

	archivoDump.Sync() // Asegurarse de que los datos se escriban en el disco

	archivoDump.Write(leerPaginasDeTabla(&proceso.TablaPaginasGlobal, 1)) //algo asi para escribir las paginas de memoria
	archivoDump.Sync()

	clientUtils.Logger.Info("Dump de memoria creado exitosamente:", "archivo", archivoDump.Name())
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Dump de memoria creado exitosamente"))
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
			return procesos[i]
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

func asignarMemoria(pid int, instrucciones []string, size int) bool {
	clientUtils.Logger.Info("Asignando memoria al proceso", "pid", pid, "tamaño", size)
	pageSize := globalsMemoria.MemoriaConfig.PageSize
	numLevels := globalsMemoria.MemoriaConfig.NumberOfLevels

	if pid < 0 {
		clientUtils.Logger.Error("PID negativo no permitido")
		return false
	}

	// Si hay concurrencia en ProcesosEnMemoria, deberías protegerlo con mutex.
	// globalsMemoria.MutexProcesos.Lock()
	// defer globalsMemoria.MutexProcesos.Unlock()

	nuevoProceso := globalsMemoria.Proceso{Pid: pid,
		Size:               size,
		Instrucciones:      instrucciones,
		TablaPaginasGlobal: globalsMemoria.NewTablaPaginas(1),
	}
	globalsMemoria.ProcesosEnMemoria = append(globalsMemoria.ProcesosEnMemoria, &nuevoProceso)

	totalPaginas := (size + pageSize - 1) / pageSize
	marcosAsignados := []int{}

	for i := 0; i < totalPaginas; i++ {
		marco := buscarMarcoLibre()
		if marco == -1 {
			clientUtils.Logger.Error("No hay marcos libres disponibles para asignar memoria")
			rollback(&nuevoProceso, marcosAsignados)
			return false
		}
		marcosAsignados = append(marcosAsignados, marco)
		pagina := globalsMemoria.NewPagina(marco, true, true, true)

		inicio := i * pageSize
		fin := min((i+1)*pageSize, size)

		for j := inicio; j < fin; j++ {
			direccionFisica := marco*pageSize + (j - inicio)
			if direccionFisica < 0 || direccionFisica >= len(globalsMemoria.MemoriaUsuario) {
				clientUtils.Logger.Error("Acceso fuera de rango a MemoriaUsuario", "direccionFisica", direccionFisica)
				rollback(&nuevoProceso, marcosAsignados)
				return false
			}
			globalsMemoria.MemoriaUsuario[direccionFisica] = 1
		}

		if err := insertarPaginaEnJerarquia(&nuevoProceso.TablaPaginasGlobal, &pagina, i, numLevels); err {
			clientUtils.Logger.Error("Error al insertar página en jerarquía", "error", err)
			rollback(&nuevoProceso, marcosAsignados)
			return false
		}
	}

	return true
}

func insertarPaginaEnJerarquia(tabla *globalsMemoria.TablaPaginas, pagina *globalsMemoria.Pagina, nroPagina int, niveles int) bool {
	actual := tabla

	for nivel := 1; nivel < niveles; nivel++ {
		indice := calcularIndice(nroPagina, nivel)

		// Validación primero
		if indice < 0 || indice >= len(actual.Entradas) {
			clientUtils.Logger.Error("Índice fuera de rango en nivel intermedio", "indice", indice, "nivel", nivel, "nroPagina", nroPagina)
			return true
		}

		siguiente := actual.Entradas[indice]
		if siguiente == nil {
			nuevaTabla := globalsMemoria.NewTablaPaginas(nivel + 1)
			tablaPtr := &nuevaTabla
			actual.Entradas[indice] = tablaPtr
			actual = tablaPtr

			clientUtils.Logger.Debug("Creada nueva tabla en nivel", "nivel", nivel, "indice", indice)
		} else {
			tablaExistente, esTabla := siguiente.(*globalsMemoria.TablaPaginas)
			if !esTabla {
				clientUtils.Logger.Error("Se esperaba tabla, pero se encontró otro tipo", "nivel", nivel, "indice", indice)
				return true
			}
			actual = tablaExistente
		}
	}

	// Último nivel (donde insertamos la página)
	indiceFinal := calcularIndice(nroPagina, niveles)

	if indiceFinal < 0 || indiceFinal >= len(actual.Entradas) {
		clientUtils.Logger.Error("Índice fuera de rango en nivel final", "indiceFinal", indiceFinal, "nivel", niveles, "nroPagina", nroPagina)
		return true
	}

	if _, yaExiste := actual.Entradas[indiceFinal].(*globalsMemoria.Pagina); yaExiste {
		clientUtils.Logger.Warn("Ya había una página en este índice, será sobrescrita", "indiceFinal", indiceFinal)
	}

	actual.Entradas[indiceFinal] = pagina
	clientUtils.Logger.Debug("Página insertada correctamente", "pagina", pagina.Marco, "indiceFinal", indiceFinal, "nroPagina", nroPagina, "nivel", niveles, "tipo de entrada", reflect.TypeOf(actual.Entradas[indiceFinal]))

	return false
}

func calcularIndice(nroPagina, nivel int) int {
	exponente := globalsMemoria.MemoriaConfig.NumberOfLevels - nivel
	divisor := 1
	for i := 0; i < exponente; i++ {
		divisor *= globalsMemoria.MemoriaConfig.EntriesPerPage
	}
	indice := (nroPagina / divisor) % globalsMemoria.MemoriaConfig.EntriesPerPage

	if indice < 0 || indice >= globalsMemoria.MemoriaConfig.EntriesPerPage {
		clientUtils.Logger.Error("Índice calculado fuera de rango", "indice", indice, "nroPagina", nroPagina, "nivel", nivel)
		return -1 // Indicador de error
	}

	return indice
}

func buscarMarcoLibre() int {
	clientUtils.Logger.Debug("Buscando marco libre")
	globalsMemoria.MutexBitmapMarcosLibres.Lock()
	defer globalsMemoria.MutexBitmapMarcosLibres.Unlock()
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
func rollback(proceso *globalsMemoria.Proceso, marcos []int) {
	globalsMemoria.MutexBitmapMarcosLibres.Lock()
	for _, m := range marcos {
		globalsMemoria.BitmapMarcosLibres[m] = true
	}
	globalsMemoria.MutexBitmapMarcosLibres.Unlock()

	liberarTabla(&proceso.TablaPaginasGlobal, 1)
}

func liberarTabla(tabla *globalsMemoria.TablaPaginas, nivelActual int) {
	for _, entrada := range tabla.Entradas {
		if entrada == nil {
			return
		}
		if nivelActual == globalsMemoria.MemoriaConfig.NumberOfLevels {
			// Es una página real
			pagina, ok := entrada.(*globalsMemoria.Pagina)

			if ok && pagina.Presencia {

				pagina.MutexPagina.Lock()
				globalsMemoria.MutexBitmapMarcosLibres.Lock()
				globalsMemoria.BitmapMarcosLibres[pagina.Marco] = true
				globalsMemoria.MutexBitmapMarcosLibres.Unlock()

				pagina.Presencia = false // Marcar la página como no válida

				pagina.MutexPagina.Unlock()
			}

		} else {
			subtabla, ok := entrada.(*globalsMemoria.TablaPaginas)
			if ok {
				liberarTabla(subtabla, nivelActual+1)
			}

		}
	}
}

func leerPaginasDeTabla(tabla *globalsMemoria.TablaPaginas, nivelActual int) []byte {
	var paginasEnMemoria []byte

	for _, entrada := range tabla.Entradas {
		if entrada == nil {
			continue
		}

		if nivelActual == globalsMemoria.MemoriaConfig.NumberOfLevels {
			pagina, ok := entrada.(*globalsMemoria.Pagina)
			if ok && pagina.Presencia {
				pagina.MutexPagina.Lock()
				marco := pagina.Marco
				inicio := marco * globalsMemoria.MemoriaConfig.PageSize
				fin := inicio + globalsMemoria.MemoriaConfig.PageSize
				paginasEnMemoria = append(paginasEnMemoria, globalsMemoria.MemoriaUsuario[inicio:fin]...)
				pagina.MutexPagina.Unlock()
			}
		} else {
			subtabla, ok := entrada.(*globalsMemoria.TablaPaginas)
			if ok {
				subPaginas := leerPaginasDeTabla(subtabla, nivelActual+1)
				paginasEnMemoria = append(paginasEnMemoria, subPaginas...)
			}
		}
	}

	return paginasEnMemoria
}

func EspacioLibre() int {
	var marcosLibres int = countMarcosLibres()
	var espacioLibre int = marcosLibres * globalsMemoria.MemoriaConfig.PageSize
	return espacioLibre
}
