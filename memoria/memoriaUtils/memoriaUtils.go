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

	clientUtils.Logger.Info("[Memoria] Configuración de memoria cargada correctamente", "config", config)

	// crea el archivo swapfile.bin si no existe
	if _, err := os.Stat(config.SwapfilePath); os.IsNotExist(err) {
		file, err := os.Create(config.SwapfilePath)
		if err != nil {
			panic("Error al crear swapfile: " + err.Error())
		}
		defer file.Close()
	}
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

	globalsMemoria.ProcesosEnMemoria = make([]globalsMemoria.Proceso, 0)

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
	clientUtils.Logger.Info("Desplazamiento recibido", "valor", desplazamiento)

	// Buscar proceso
	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado", "pid", pid)
		http.Error(w, "PID no existe", http.StatusNotFound)
		return
	}

	// Acceder recursivamente a las tablas de páginas
	actual := &proceso.TablaPaginasGlobal

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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(direccionFisica)))
}

func LeerPagina(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[Memoria] Petición para leer una página recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)

	clientUtils.Logger.Debug("Los valores recibidos en leerPagina", "valores: ", pedido.Valores)

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
	proceso.Metricas.LecturasDeMemoria++
	clientUtils.Logger.Info("Página leída", "pid", pid, "marco", marco, "contenido", contenido)

	w.Write([]byte(contenido))
	w.WriteHeader(http.StatusOK)
}

func EscribirPagina(w http.ResponseWriter, r *http.Request) {

	clientUtils.Logger.Info("[Memoria] Petición para escribir una página recibida desde CPU")

	pedido := serverUtils.RecibirPaquetes(w, r)
	clientUtils.Logger.Debug("Los valores recibidos en escribirPagina", "valores: ", pedido.Valores)
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
	proceso.Metricas.EscriturasDeMemoria++
	clientUtils.Logger.Info("Página escrita", "pid", pid, "marco", marco, "tamaño", tamanioEnviado)
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
	w.Write([]byte{contenido})
	w.WriteHeader(http.StatusOK)
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

	paginas := leerPaginasDeTabla(&proceso.TablaPaginasGlobal, 1)

	/*if len(paginas) == 0 {
		clientUtils.Logger.Error("No se encontraron páginas para swappear:", "pid", pid)
		http.Error(w, "No se encontraron páginas para swappear", http.StatusNotFound)
		return
	}*/

	//leer la tabla de paginas y escribir en el swapfile
	swapFile, err := os.OpenFile(globalsMemoria.MemoriaConfig.SwapfilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		clientUtils.Logger.Error("Error al abrir swapfile:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	//leer las paginas de memoria del proceso y escribirlas en el swapfile
	// considerando que en el swapfile se guardan paginas enteras y en memoria existe una estructura que tiene los pid, pagina y marco de lo swapeado

	_, err = swapFile.Write(paginas)
	if err != nil {
		clientUtils.Logger.Error("Error al escribir en swapfile:", "error", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	//mutex de tablaswap
	globalsMemoria.MutexTablaSwap.Lock()
	defer globalsMemoria.MutexTablaSwap.Unlock()
	globalsMemoria.TablaSwap[pid] = append(globalsMemoria.TablaSwap[pid], globalsMemoria.ProcesoEnSwap{

		Pid:    pid,
		Offset: globalsMemoria.SiguienteOffsetLibre,
		Size:   len(paginas) * globalsMemoria.MemoriaConfig.PageSize, //chequear si es correcto

	})
	globalsMemoria.SiguienteOffsetLibre += int64(globalsMemoria.MemoriaConfig.PageSize) * int64(len(paginas))

	// Actualizar el bitmap de marcos libres y el bit de validez de las páginas
	liberarTabla(&proceso.TablaPaginasGlobal, 1)

	defer swapFile.Close()

	proceso.Metricas.BajadasASwap++

	clientUtils.Logger.Info("Proceso suspendido:", "pid", pid)
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
	clientUtils.Logger.Debug("antes del mutex de TablaSwap", "pid", pid)
	globalsMemoria.MutexTablaSwap.Lock()
	entradas, ok := globalsMemoria.TablaSwap[pid]
	clientUtils.Logger.Debug("dentro del mutex de TablaSwap", "pid", pid)
	globalsMemoria.MutexTablaSwap.Unlock()
	clientUtils.Logger.Debug("TablaSwap desbloqueada", "pid", pid)
	clientUtils.Logger.Debug("Entradas de swap encontradas", "pid", pid, "entradas", len(entradas))

	if !ok {
		http.Error(w, "PID no encontrado en TablaSwap", http.StatusNotFound)
		return
	}

	swapFile, err := os.Open(globalsMemoria.MemoriaConfig.SwapfilePath)
	if err != nil {
		http.Error(w, "Error al abrir swapfile", http.StatusInternalServerError)
		return
	}
	clientUtils.Logger.Debug("Swapfile abierto", "path", globalsMemoria.MemoriaConfig.SwapfilePath)
	defer swapFile.Close()

	if len(entradas) == 0 {
		if !reAsignarMemoria(pid, make([]byte, globalsMemoria.MemoriaConfig.PageSize), 0) {
			http.Error(w, "Error al reasignar memoria", http.StatusInternalServerError)
			return
		}
	}

	for numeroPagina, entrada := range entradas {
		pagina := make([]byte, globalsMemoria.MemoriaConfig.PageSize)
		clientUtils.Logger.Debug("Leyendo pagina del swapfile", "pid", pid, "numeroPagina", numeroPagina, "offset", entrada.Offset)

		_, err := swapFile.ReadAt(pagina, entrada.Offset)
		if err != nil {
			clientUtils.Logger.Error("Error al leer swapfile:", "error", err)
			http.Error(w, "Error al leer swapfile", http.StatusInternalServerError)
			return
		}

		if !reAsignarMemoria(pid, pagina, numeroPagina) {
			http.Error(w, "Error al reasignar memoria", http.StatusInternalServerError)
			return
		}

	}

	proceso.Metricas.SubidasAMemoria++

	clientUtils.Logger.Info("Proceso desuspendido exitosamente:", "pid", pid)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso desuspendido exitosamente"))

}
func reAsignarMemoria(pid int, contenidoPagina []byte, numeroPagina int) bool {
	clientUtils.Logger.Info("Reasignando memoria al proceso", "pid", pid, "tamaño", len(contenidoPagina))

	if pid < 0 {
		return false
	}
	if len(contenidoPagina) > EspacioLibre() {
		clientUtils.Logger.Error("Espacio insuficiente para reasignar memoria al proceso", "pid", pid, "tamaño requerido", len(contenidoPagina))
		return false
	}
	// Si hay concurrencia en ProcesosEnMemoria, deberías protegerlo con mutex.
	clientUtils.Logger.Debug("Espacio libre disponible", "espacio", EspacioLibre())

	clientUtils.Logger.Debug("Mutex de procesos bloqueado", "pid", pid)
	proceso := buscarProceso(pid)
	if proceso == nil {
		clientUtils.Logger.Error("Proceso no encontrado:", "pid especifico", pid)

		return false
	}
	clientUtils.Logger.Debug("Proceso encontrado", "pid", pid, "tamaño", proceso.Size)

	clientUtils.Logger.Debug("Mutex de bitmap de marcos libres bloqueado", "pid", pid)
	marcoLibre := buscarMarcoLibre()
	if marcoLibre == -1 {
		clientUtils.Logger.Error("No se encontró un marco libre para reasignar memoria", "pid", pid)

		return false
	}
	clientUtils.Logger.Debug("Marco libre encontrado", "marcoLibre", marcoLibre)

	// Escribe el marco libre con el contenido de la pagina
	copy(globalsMemoria.MemoriaUsuario[marcoLibre*globalsMemoria.MemoriaConfig.PageSize:], contenidoPagina)
	clientUtils.Logger.Debug("Contenido de la página escrito en memoria", "marcoLibre", marcoLibre, "tamaño", len(contenidoPagina))

	// Reescribir el marco de memoria a la entrada de la ultima tabla mutinivel y marcarlo como válido, presente y modificado
	if len(proceso.TablaPaginasGlobal.Entradas) == 0 {
		clientUtils.Logger.Error("Tabla de páginas del proceso está vacía, no se puede reasignar memoria", "pid", pid)

		return false
	}

	//ahora tengo que buscar en base a la pagina espeficifica que tengo dentro de las tablas y reasignale el marco y los bits
	//aca deberia hacer el calculo raro para saber a que entrada de cada tabla tengo que entrar para poder modificar la ultima
	reasignarPaginaEnJerarquia(pid, numeroPagina, marcoLibre)
	clientUtils.Logger.Debug("Página reasignada en jerarquía", "pid", pid, "numeroPagina", numeroPagina, "marcoLibre", marcoLibre)

	globalsMemoria.BitmapMarcosLibres[marcoLibre] = false // Marcar el marco como ocupado

	//  limpiar la entradas swap
	globalsMemoria.MutexTablaSwap.Lock()
	delete(globalsMemoria.TablaSwap, pid)
	clientUtils.Logger.Debug("Tabla de swap limpiada para el proceso", "pid", pid)
	globalsMemoria.MutexTablaSwap.Unlock()
	return true
}

func reasignarPaginaEnJerarquia(pid int, nroPagina int, marcoLibre int) bool {
	// Navegar por la jerarquía hasta el nivel correspondiente
	actual := &globalsMemoria.ProcesosEnMemoria[pid].TablaPaginasGlobal
	for nivel := 1; nivel < globalsMemoria.MemoriaConfig.NumberOfLevels; nivel++ {
		indice := calcularIndice(nroPagina, nivel)
		siguiente := actual.Entradas[indice]
		if siguiente == nil {
			clientUtils.Logger.Error("Error: no se encontró la tabla en el nivel esperado", "nivel", nivel, "nroPagina", nroPagina)
			return true
		}
		tablaExistente, esTabla := siguiente.(*globalsMemoria.TablaPaginas)
		if !esTabla {
			clientUtils.Logger.Error("Error: la entrada no debería ser una página", "nivel", nivel, "nroPagina", nroPagina)
			return true
		}
		actual = tablaExistente
	}

	// Nivel N: insertar página real
	indiceFinal := calcularIndice(nroPagina, globalsMemoria.MemoriaConfig.NumberOfLevels-1)

	pagina, ok := actual.Entradas[indiceFinal].(*globalsMemoria.Pagina)
	if ok {
		clientUtils.Logger.Info("Reasignando página en nivel final", "nroPagina", nroPagina, "marcoLibre", marcoLibre)
		pagina.MutexPagina.Lock()
		defer pagina.MutexPagina.Unlock()
		pagina.Marco = marcoLibre

		pagina.Presencia = true

		pagina.BitModificado = true
	} else {
		clientUtils.Logger.Error("Error: la entrada final no debería ser una página", "nroPagina", nroPagina)
	}

	return false // No hubo error
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

	// Escribir instrucciones del proceso con ese pid que esta en memoria en el archivo de dump
	// no recuerdo si no estan tambien guardadas en memoria y referenciadas por las tabla de paginas
	for i := 0; i < len(proceso.Instrucciones); i++ {
		instruccion := proceso.Instrucciones[i]
		if _, err := archivoDump.WriteString(instruccion + "\n"); err != nil {
			clientUtils.Logger.Error("Error al escribir en el archivo de dump:", "error", err)
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}
	}
	archivoDump.Sync() // Asegurarse de que los datos se escriban en el disco

	archivoDump.Write(leerPaginasDeTabla(&proceso.TablaPaginasGlobal, 1)) //algo asi para escribir las paginas de memoria
	archivoDump.Sync()

	liberarTabla(&proceso.TablaPaginasGlobal, 1) // Liberar la tabla de páginas del proceso

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
			return &procesos[i]
		}
	}

	return nil
}

func countMarcosLibres() int {
	clientUtils.Logger.Info("Contando marcos libres")
	globalsMemoria.MutexContadorMarcosLibres.Lock()
	clientUtils.Logger.Debug("Paso el lock mutexContadorMarcosLibres")
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

	if pid >= len(globalsMemoria.ProcesosEnMemoria) {
		nuevo := make([]globalsMemoria.Proceso, pid-len(globalsMemoria.ProcesosEnMemoria)+1)
		globalsMemoria.ProcesosEnMemoria = append(globalsMemoria.ProcesosEnMemoria, nuevo...)
	}

	proceso := &globalsMemoria.ProcesosEnMemoria[pid]
	proceso.Pid = pid
	proceso.Size = size
	proceso.Instrucciones = instrucciones
	proceso.TablaPaginasGlobal = globalsMemoria.NewTablaPaginas(1)

	totalPaginas := (size + pageSize - 1) / pageSize
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
		fin := min((i+1)*pageSize, size)

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
				// Leer contenido del marco físico en memoria
				for i := 0; i < globalsMemoria.MemoriaConfig.PageSize; i++ {
					contenido := globalsMemoria.MemoriaUsuario[marco]
					paginasEnMemoria = append(paginasEnMemoria, contenido)
					marco++
				}
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
