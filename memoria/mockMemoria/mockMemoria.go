package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	memoriaUtils "github.com/sisoputnfrba/tp-golang/memoria/memoriaUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

// --- Estructuras multinivel ---

type Pagina struct {
	Contenido []byte
	Marco     int // número de marco asignado en Memoria
}

type TablaPaginas struct {
	Entradas map[int]interface{} // Puede ser *TablaPaginas o *Pagina
}

// Variables globales para mock
var tablaRaiz *TablaPaginas

func main() {

	// Inicializa el logger que usará todo el módulo Memoria
	clientUtils.ConfigurarLogger("memoria.log")

	// Carga la configuración desde el archivo config.json
	globalsMemoria.MemoriaConfig = memoriaUtils.IniciarConfiguracion("config.json")

	tablaRaiz = inicializarTablaMultinivelGenerica(globalsMemoria.MemoriaConfig.NumberOfLevels, globalsMemoria.MemoriaConfig.EntriesPerPage, globalsMemoria.MemoriaConfig.PageSize)

	globalsMemoria.MemoriaUsuario = make([]byte, globalsMemoria.MemoriaConfig.MemorySize)
	globalsMemoria.BitmapMarcosLibres = make([]bool, globalsMemoria.MemoriaConfig.MemorySize/globalsMemoria.MemoriaConfig.PageSize)
	for i := range globalsMemoria.BitmapMarcosLibres {
		globalsMemoria.BitmapMarcosLibres[i] = true
	}

	// Crea el multiplexer HTTP y registra los endpoints que usará Memoria
	mux := http.NewServeMux()

	// Endpoints que reciben peticiones desde Kernel
	mux.HandleFunc("/iniciarProceso", memoriaUtils.IniciarProceso)
	mux.HandleFunc("/finalizarProceso", memoriaUtils.FinalizarProceso)
	mux.HandleFunc("/suspenderProceso", memoriaUtils.SuspenderProceso)
	mux.HandleFunc("/desuspenderProceso", memoriaUtils.DesuspenderProceso)

	// Endpoints que reciben peticiones desde CPU
	mux.HandleFunc("/obtenerConfiguracionMemoria", memoriaUtils.ObtenerConfiguracionMemoria)
	mux.HandleFunc("/siguienteInstruccion", memoriaUtils.SiguienteInstruccion)
	mux.HandleFunc("/accederMarcoUsuario", accederMarcoUsuarioMock)
	mux.HandleFunc("/readPagina", leerPaginaMock)
	mux.HandleFunc("/writePagina", escribirPaginaMock)
	mux.HandleFunc("/writeMemoria", escribirDireccionFisicaMock)
	mux.HandleFunc("/readMemoria", leerDireccionFisicaMock)

	// Levanta el servidor en el puerto definido por configuración
	direccion := fmt.Sprintf("%s:%d", globalsMemoria.MemoriaConfig.IpMemory, globalsMemoria.MemoriaConfig.PortMemory)
	fmt.Printf("[Memoria] Servidor escuchando en puerto %d...\n", globalsMemoria.MemoriaConfig.PortMemory)

	err := http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}
}

// --- Funciones auxiliares ---

func inicializarTablaMultinivelGenerica(niveles int, entradasPorTabla int, tamanioPagina int) *TablaPaginas {
	return crearNivel(0, niveles, entradasPorTabla, tamanioPagina)
}

// Función recursiva que crea la tabla o página según nivel
func crearNivel(nivelActual, niveles, entradasPorTabla, tamanioPagina int) *TablaPaginas {
	tabla := &TablaPaginas{Entradas: make(map[int]interface{})}

	for i := 0; i < entradasPorTabla; i++ {
		if nivelActual == niveles-1 {
			// Último nivel: crea página con contenido vacío
			tabla.Entradas[i] = NuevaPagina(tamanioPagina)
		} else {
			// Niveles intermedios: crea tabla multinivel recursivamente
			tabla.Entradas[i] = crearNivel(nivelActual+1, niveles, entradasPorTabla, tamanioPagina)
		}
	}

	return tabla
}

func ObtenerOCrearPagina(tabla *TablaPaginas, indices []int, tamanioPagina int) (*Pagina, error) {
	actual := tabla

	// Navega por todos los niveles excepto el último
	for i := 0; i < len(indices)-1; i++ {
		idx := indices[i]

		if siguiente, ok := actual.Entradas[idx]; ok {
			// Existe, debe ser tabla
			siguienteTabla, ok := siguiente.(*TablaPaginas)
			if !ok {
				return nil, fmt.Errorf("en índice %d, no es tabla multinivel", idx)
			}
			actual = siguienteTabla
		} else {
			// No existe, intentamos crear nueva tabla multinivel en ese índice
			nueva := &TablaPaginas{Entradas: make(map[int]interface{})}
			actual.Entradas[idx] = nueva
			actual = nueva
		}
	}

	// Ahora el último índice indica la página
	final := indices[len(indices)-1]
	if entrada, ok := actual.Entradas[final]; ok {
		// Si ya existe, debería ser página
		pagina, ok := entrada.(*Pagina)
		if !ok {
			return nil, fmt.Errorf("en índice final %d, no es página", final)
		}
		return pagina, nil
	} else {
		// No existe la página, la creamos
		nuevaPagina := NuevaPagina(tamanioPagina)
		actual.Entradas[final] = nuevaPagina
		return nuevaPagina, nil
	}
}

func NuevaPagina(tamanioPagina int) *Pagina {
	// Buscar marco libre
	for i, libre := range globalsMemoria.BitmapMarcosLibres {
		if libre {
			globalsMemoria.BitmapMarcosLibres[i] = false
			return &Pagina{
				Contenido: make([]byte, tamanioPagina),
				Marco:     i,
			}
		}
	}
	panic("No hay marcos libres")
}

func InsertarPagina(tabla *TablaPaginas, indices []int, tamanioPagina int) {
	actual := tabla
	for i := 0; i < len(indices)-1; i++ {
		idx := indices[i]
		if siguiente, ok := actual.Entradas[idx]; ok {
			actual = siguiente.(*TablaPaginas)
		} else {
			nueva := &TablaPaginas{Entradas: make(map[int]interface{})}
			actual.Entradas[idx] = nueva
			actual = nueva
		}
	}
	final := indices[len(indices)-1]
	actual.Entradas[final] = NuevaPagina(tamanioPagina)
}

func ObtenerPagina(tabla *TablaPaginas, indices []int) (*Pagina, error) {
	actual := tabla
	for i := 0; i < len(indices)-1; i++ {
		siguiente, ok := actual.Entradas[indices[i]].(*TablaPaginas)
		if !ok {
			return nil, fmt.Errorf("nivel %d no es tabla", i)
		}
		actual = siguiente
	}
	pagina, ok := actual.Entradas[indices[len(indices)-1]].(*Pagina)
	if !ok {
		return nil, fmt.Errorf("no se encontró la página")
	}
	return pagina, nil
}

// --- Funciones HTTP mock ---

func accederMarcoUsuarioMock(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[MockMemoria] Petición accederMarcoUsuario recibida")

	pedido := serverUtils.RecibirPaquetes(w, r)
	clientUtils.Logger.Debug("Valores recibidos en accederMarcoUsuarioMock", "valores:", pedido.Valores)

	if len(pedido.Valores) < 3 {
		clientUtils.Logger.Error("Paquete insuficiente")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var indices []int
	for i := 1; i < len(pedido.Valores)-1; i++ {
		val, err := strconv.Atoi(pedido.Valores[i])
		if err != nil {
			clientUtils.Logger.Error("Error al parsear índice de tabla multinivel", "indice", i, "valor", pedido.Valores[i])
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		indices = append(indices, val)
	}

	desplazamiento, err := strconv.Atoi(pedido.Valores[len(pedido.Valores)-1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear desplazamiento")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pagina, err := ObtenerOCrearPagina(tablaRaiz, indices, globalsMemoria.MemoriaConfig.PageSize)
	if err != nil {
		clientUtils.Logger.Error("No se pudo obtener o crear página", "error", err)
		http.Error(w, "No se pudo obtener o crear página", http.StatusInternalServerError)
		return
	}

	// Asumiendo que la página tiene un campo Marco int
	marco := pagina.Marco
	clientUtils.Logger.Info("Marco accedido", "pid", pid, "marco", marco, "desplazamiento", desplazamiento)

	time.Sleep(time.Duration(globalsMemoria.MemoriaConfig.MemoryDelay) * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(marco)))

}
func leerPaginaMock(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[MockMemoria] Petición leerPagina recibida")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 3 {
		clientUtils.Logger.Error("Paquete insuficiente en leerPaginaMock")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
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
		clientUtils.Logger.Error("Error al parsear tamaño")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if tamanioEnviado != globalsMemoria.MemoriaConfig.PageSize {
		clientUtils.Logger.Error("Tamaño incorrecto")
		http.Error(w, "Tamaño incorrecto", http.StatusBadRequest)
		return
	}

	// Para mock, usamos marco como el índice para acceder a la página a nivel raíz, hoja con índice "marco"
	pagina, ok := tablaRaiz.Entradas[marco].(*Pagina)
	if !ok {
		clientUtils.Logger.Error("Página no encontrada en leerPaginaMock", "marco", marco)
		http.Error(w, "Página no encontrada", http.StatusNotFound)
		return
	}

	clientUtils.Logger.Info("Página leída", "pid", pid, "marco", marco)
	w.WriteHeader(http.StatusOK)
	w.Write(pagina.Contenido)
}

func escribirPaginaMock(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[MockMemoria] Petición escribirPagina recibida")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 4 {
		clientUtils.Logger.Error("Paquete insuficiente en escribirPaginaMock")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
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
		clientUtils.Logger.Error("Error al parsear tamaño")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if tamanioEnviado != globalsMemoria.MemoriaConfig.PageSize {
		clientUtils.Logger.Error("Tamaño incorrecto")
		http.Error(w, "Tamaño incorrecto", http.StatusBadRequest)
		return
	}

	// Obtener página a escribir
	pagina, ok := tablaRaiz.Entradas[marco].(*Pagina)
	if !ok {
		clientUtils.Logger.Error("Página no encontrada en escribirPaginaMock", "marco", marco)
		http.Error(w, "Página no encontrada", http.StatusNotFound)
		return
	}

	// Leer contenido para escribir (desde posición 3 en adelante)
	if len(pedido.Valores)-3 != tamanioEnviado {
		clientUtils.Logger.Error("Cantidad de bytes a escribir no coincide con tamaño de página")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	for i := 0; i < tamanioEnviado; i++ {
		valor, err := strconv.Atoi(pedido.Valores[3+i])
		if err != nil {
			clientUtils.Logger.Error("Error al parsear byte a escribir", "indice", i, "valor", pedido.Valores[3+i])
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		pagina.Contenido[i] = byte(valor)
	}

	clientUtils.Logger.Info("Página escrita", "pid", pid, "marco", marco)
	w.WriteHeader(http.StatusOK)
}

func escribirDireccionFisicaMock(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[MockMemoria] Petición escribirDireccionFisica recibida")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 3 {
		clientUtils.Logger.Error("Paquete insuficiente en escribirDireccionFisicaMock")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	direccionFisica, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear dirección física")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	valorByte, err := strconv.Atoi(pedido.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear valor byte")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if direccionFisica < 0 || direccionFisica >= len(globalsMemoria.MemoriaUsuario) {
		clientUtils.Logger.Error("Dirección física fuera de rango")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	globalsMemoria.MemoriaUsuario[direccionFisica] = byte(valorByte)

	clientUtils.Logger.Info("Byte escrito en memoria física", "pid", pid, "direccion", direccionFisica, "valor", valorByte)
	w.WriteHeader(http.StatusOK)
}

func leerDireccionFisicaMock(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("[MockMemoria] Petición leerDireccionFisica recibida")

	pedido := serverUtils.RecibirPaquetes(w, r)
	if len(pedido.Valores) < 2 {
		clientUtils.Logger.Error("Paquete insuficiente en leerDireccionFisicaMock")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(pedido.Valores[0])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	direccionFisica, err := strconv.Atoi(pedido.Valores[1])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear dirección física")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if direccionFisica < 0 || direccionFisica >= len(globalsMemoria.MemoriaUsuario) {
		clientUtils.Logger.Error("Dirección física fuera de rango")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	valor := globalsMemoria.MemoriaUsuario[direccionFisica]

	clientUtils.Logger.Info("Byte leído de memoria física", "pid", pid, "direccion", direccionFisica, "valor", valor)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{valor})
}
