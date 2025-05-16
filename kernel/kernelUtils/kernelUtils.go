package kernelUtils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	globalskernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

// Listas globales para almacenar las CPUs e IOs conectadas

var cpusLibres []Cpu
var cpusOcupadas []Cpu
var iosRegistradas map[string]Io

// PID para nuevos procesos
var proximoPID uint = 0
var Plp PlanificadorLargoPlazo

func IniciarConfiguracion(filePath string) *globalskernel.Config {
	config := &globalskernel.Config{} // Aca creamos el contenedor donde irá el JSON

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

// TODO: implementar inicializacion de pcp
func InciarPcp() PlanificadorCortoPlazo

func InciarPlp() PlanificadorLargoPlazo {
	var estrategia NewAlgorithmEstrategy
	algoritmo := globalskernel.KernelConfig.ReadyIngressAlgorithm
	if algoritmo == "FIFO" {
		estrategia = FIFOEstrategy{}
	} else if algoritmo == "PMCP" {
		estrategia = PMCPEstrategy{}
	}

	return PlanificadorLargoPlazo{newAlgorithmEstrategy: estrategia, pcp: InciarPcp()}
}

// Estructura para representar CPUs e IOs conectados al Kernel
type Cpu struct {
	Indentificador string `json:"identificador"`
	Ip             string `json:"ip"`
	Puerto         int    `json:"puerto"`
	PIDenEjecucion uint
}

func (cpu Cpu) enviarProceso(PID uint, PC uint) {
	valores := []string{strconv.Itoa(int(PID)), strconv.Itoa(int(PC))}
	paquete := clientUtils.Paquete{Valores: valores}

	//Mandamos el PID y PC al endpoint de CPU
	endpoint := "recibirProceso"

	clientUtils.EnviarPaquete(cpu.Ip, cpu.Puerto, endpoint, paquete)
}

type Io struct {
	Nombre            string
	Ip                string
	Puerto            int
	ocupada           bool
	conectada         bool
	procesosEsperando []PedidoIo
}

type PedidoIo struct {
	PID  uint
	time int
}

func (io *Io) TieneProcesosEsperando() bool {
	return len(io.procesosEsperando) > 0
}

func (io *Io) SacarProximoProceso() (PedidoIo, bool) {
	if len(io.procesosEsperando) == 0 {
		var vacio PedidoIo
		return vacio, false
	}
	prox := io.procesosEsperando[0]
	io.procesosEsperando = io.procesosEsperando[1:]
	return prox, true
}

func (io Io) enviarProceso(PID uint, time int) {
	valores := []string{strconv.Itoa(int(PID)), strconv.Itoa(time)}
	paquete := clientUtils.Paquete{Valores: valores}

	//Mandamos el PID y tiempo al endpoint de IO
	endpoint := "recibirPeticion"

	clientUtils.EnviarPaquete(io.Ip, io.Puerto, endpoint, paquete)
}

func ObtenerCpuLibre() (*Cpu, bool) {
	if len(cpusLibres) == 0 {
		return nil, false
	}

	// Tomamos la primera CPU libre
	cpu := &cpusLibres[0]

	// Sacamos la CPU de la lista de libres
	cpusLibres = cpusLibres[1:]

	// Agregamos la misma CPU a la lista de ocupadas
	cpusOcupadas = append(cpusOcupadas, *cpu)

	return cpu, true
}

//-------------- Estructuras generales para el manejo de estados -------------------

type MetricasDeEstado struct {
	newCount         uint
	readyCount       uint
	execCount        uint
	bloquedCount     uint
	suspReadyCount   uint
	suspBlockedCount uint
	exitCount        uint
}

type MetricasDeTiempo struct {
	newTime         float64
	readyTime       float64
	execTime        float64
	bloquedTime     float64
	suspReadyTime   float64
	suspBlockedTime float64
	exitTime        float64
}

type PCB struct {
	PID                uint
	PC                 uint
	ProcessSize        uint
	FilePath           string
	ME                 MetricasDeEstado
	MT                 MetricasDeTiempo
	timeInCurrentState time.Time
}

type PCBList struct {
	elementos []PCB
}

func (p *PCBList) Agregar(proceso PCB) {
	p.elementos = append(p.elementos, proceso)
}

func (p *PCBList) SacarProximoProceso() (PCB, bool) {
	var cero PCB
	if len(p.elementos) == 0 {
		return cero, false
	}
	primero := p.elementos[0]
	p.elementos = p.elementos[1:]
	return primero, true
}

func (p *PCBList) EliminarProcesoPorPID(pid uint) {
	for i, pcb := range p.elementos {
		if pcb.PID == pid {
			// Eliminamos el elemento encontrado de la lista
			p.elementos = append(p.elementos[:i], p.elementos[i+1:]...)
			break
		}
	}
}

func (p *PCBList) BuscarPorPID(pid uint) (PCB, bool) {
	for _, pcb := range p.elementos {
		if pcb.PID == pid {
			return pcb, true
		}
	}
	var cero PCB
	return cero, false
}

func (p *PCBList) BuscarYSacarPorPID(pid uint) (PCB, bool) {
	for i, pcb := range p.elementos {
		if pcb.PID == pid {
			// Sacamos el elemento de la lista
			p.elementos = append(p.elementos[:i], p.elementos[i+1:]...)
			return pcb, true
		}
	}
	var cero PCB
	return cero, false
}

func (p *PCBList) Vacia() bool {
	return len(p.elementos) == 0
}

func (p *PCBList) VizualizarProximo() PCB {
	return p.elementos[0] //Si la lista está vacia puede dar un panic
}

func (p *PCBList) OrdenarPorPMC() {
	sort.Slice(p.elementos, func(i, j int) bool {
		return p.elementos[i].ProcessSize < p.elementos[j].ProcessSize
	})
}

func (p *PCB) timeInState() float64 {
	return time.Since(p.timeInCurrentState).Seconds()
}

//---------------- PLANIFICADOR LARGO PLAZO ---------------------------------------------

type NewAlgorithmEstrategy interface {
	manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo)
	manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo)
}

type FIFOEstrategy struct {
}

func (f FIFOEstrategy) manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo) {
	plp.newState.Agregar(nuevoProceso)
}

func (f FIFOEstrategy) manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo) {
	// chequea una copia del mismo, si puede irse lo desencola
	proximoProceso := plp.newState.VizualizarProximo()
	if plp.EnviarPedidoMemoria(proximoProceso) {
		plp.EnviarProcesoAReady(proximoProceso)
		plp.newState.SacarProximoProceso()
	}
	// Si sale mal
	// aca podria ir un log de que todavia no hay espacio suficiente

}

type PMCPEstrategy struct {
}

func (p PMCPEstrategy) manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo) {
	plp.intentarInicializar(nuevoProceso)
}

func (p PMCPEstrategy) manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo) {
	plp.newState.OrdenarPorPMC()
	proximoProceso := plp.newState.VizualizarProximo()
	if plp.EnviarPedidoMemoria(proximoProceso) {
		plp.EnviarProcesoAReady(proximoProceso)
		plp.newState.SacarProximoProceso()
	}
}

type PlanificadorLargoPlazo struct {
	newState              PCBList
	exitState             PCBList
	newAlgorithmEstrategy NewAlgorithmEstrategy
	pcp                   PlanificadorCortoPlazo
}

func (plp *PlanificadorLargoPlazo) RecibirNuevoProceso(nuevoProceso PCB) {
	nuevoProceso.timeInCurrentState = time.Now()
	if plp.newState.Vacia() {
		plp.intentarInicializar(nuevoProceso)
	} else {
		plp.newAlgorithmEstrategy.manejarIngresoDeProceso(nuevoProceso, plp)
	}
}

func (plp PlanificadorLargoPlazo) intentarInicializar(nuevoProceso PCB) {
	if plp.EnviarPedidoMemoria(nuevoProceso) {
		plp.EnviarProcesoAReady(nuevoProceso)
	} else {
		plp.newState.Agregar(nuevoProceso)
	}
}

func (plp PlanificadorLargoPlazo) EnviarProcesoAReady(proceso PCB) {

	// Log del cambio de estado NEW → READY
	clientUtils.Logger.Info(fmt.Sprintf(`Cambio de Estado: "## (%d) Pasa del estado NEW al estado READY"`, proceso.PID))

	proceso.ME.newCount++
	proceso.MT.newTime += proceso.timeInState()

	proceso.timeInCurrentState = time.Now()
	plp.pcp.RecibirProceso(proceso)
}

func (plp *PlanificadorLargoPlazo) FinalizarProceso(proceso PCB) {

	if plp.EnviarFinalizacionMemoria(proceso) {

		// Registramos el tiempo en el que el proceso entra en EXIT
		proceso.timeInCurrentState = time.Now()
		proceso.ME.exitCount++

		// Confirmamos la transición de EXEC → EXIT
		clientUtils.Logger.Info(fmt.Sprintf(`Cambio de Estado: "## (%d) Pasa del estado EXEC al estado EXIT"`, proceso.PID))

		// TODO: aca iria mediano plazo chequear los susps ready (Checkpoint 3)
		// si no chequea new

		// Eliminamos el proceso de la lista de EXEC
		plp.pcp.execState.EliminarProcesoPorPID(proceso.PID)

		// Se registra en la lista de EXIT para registrar el cambio de estado
		plp.exitState.Agregar(proceso)

		plp.loggearMetricas(proceso)

		plp.newAlgorithmEstrategy.manejarLiberacionDeProceso(plp)

	} else {
		// Logueamos el error si Memoria rechazó la finalización
		clientUtils.Logger.Error(fmt.Sprintf("Error: Memoria no aceptó finalizar el proceso PID %d", proceso.PID))
	}
}

func (plp PlanificadorLargoPlazo) loggearMetricas(proceso PCB) {
	proceso.MT.exitTime += proceso.timeInState()

	clientUtils.Logger.Info(fmt.Sprintf("Proceso finalizado - PID: %d", proceso.PID))

	clientUtils.Logger.Info("++ Métricas de Estado:")
	clientUtils.Logger.Info(fmt.Sprintf("  NEW:          %d", proceso.ME.newCount))
	clientUtils.Logger.Info(fmt.Sprintf("  READY:        %d", proceso.ME.readyCount))
	clientUtils.Logger.Info(fmt.Sprintf("  EXEC:         %d", proceso.ME.execCount))
	clientUtils.Logger.Info(fmt.Sprintf("  BLOCKED:      %d", proceso.ME.bloquedCount))
	clientUtils.Logger.Info(fmt.Sprintf("  SUSP_READY:   %d", proceso.ME.suspReadyCount))
	clientUtils.Logger.Info(fmt.Sprintf("  SUSP_BLOCKED: %d", proceso.ME.suspBlockedCount))
	clientUtils.Logger.Info(fmt.Sprintf("  EXIT:         %d", proceso.ME.exitCount))

	clientUtils.Logger.Info("-- Métricas de Tiempo (en segundos):")
	clientUtils.Logger.Info(fmt.Sprintf("  NEW:          %.2f", proceso.MT.newTime))
	clientUtils.Logger.Info(fmt.Sprintf("  READY:        %.2f", proceso.MT.readyTime))
	clientUtils.Logger.Info(fmt.Sprintf("  EXEC:         %.2f", proceso.MT.execTime))
	clientUtils.Logger.Info(fmt.Sprintf("  BLOCKED:      %.2f", proceso.MT.bloquedTime))
	clientUtils.Logger.Info(fmt.Sprintf("  SUSP_READY:   %.2f", proceso.MT.suspReadyTime))
	clientUtils.Logger.Info(fmt.Sprintf("  SUSP_BLOCKED: %.2f", proceso.MT.suspBlockedTime))
	clientUtils.Logger.Info(fmt.Sprintf("  EXIT:         %.2f", proceso.MT.exitTime))
}

// pedido de inicialización de proceso devuelve si Memoria tiene espacio suficiente para inicializarlo
func (plp PlanificadorLargoPlazo) EnviarPedidoMemoria(nuevoProceso PCB) bool {

	// Creamos el contenido del paquete con lo que la Memoria necesita:
	// PID, Ruta al pseudocódigo, y Tamaño del proceso
	valores := []string{
		strconv.Itoa(int(nuevoProceso.PID)),
		nuevoProceso.FilePath,
		strconv.Itoa(int(nuevoProceso.ProcessSize)),
	}

	// Construimos el paquete
	paquete := clientUtils.Paquete{Valores: valores}

	// Obtenemos IP y puerto de Memoria desde la config global del Kernel
	ip := globalskernel.KernelConfig.IpMemory
	puerto := globalskernel.KernelConfig.PortMemory
	endpoint := "iniciarProceso"

	resp := clientUtils.EnviarPaqueteConRespuesta(ip, puerto, endpoint, paquete)

	// Validamos la respuesta (por ahora asumimos éxito si hay respuesta 200 OK)
	if resp != nil && resp.StatusCode == http.StatusOK {
		clientUtils.Logger.Info(fmt.Sprintf("Proceso PID %d enviado a Memoria correctamente", nuevoProceso.PID))
		return true
	}

	clientUtils.Logger.Warn(fmt.Sprintf("Memoria rechazó el proceso PID %d o hubo un error en la conexión", nuevoProceso.PID))
	return false
}

// envio del aviso de finalizacion a memoria
func (plp PlanificadorLargoPlazo) EnviarFinalizacionMemoria(procesoTernminado PCB) bool {

	// Creamos paquete que contenga solo el PID
	valores := []string{strconv.Itoa(int(procesoTernminado.PID))}
	paquete := clientUtils.Paquete{Valores: valores}

	// Fijamos la direccion del endpoint de memoria
	ip := globalskernel.KernelConfig.IpMemory
	puerto := globalskernel.KernelConfig.PortMemory
	endpoint := "finalizarProceso"

	//Usamos EnviarPaqueteConRespuesta que devuelve la respuesta del servidor
	resp := clientUtils.EnviarPaqueteConRespuesta(ip, puerto, endpoint, paquete)
	if resp != nil && resp.StatusCode == http.StatusOK {
		clientUtils.Logger.Info(fmt.Sprintf("Proceso PID %d finalizado correctamente en memoria", procesoTernminado.PID))
		return true
	}

	//Si no responde con 200 OK, lo logueamos como advertencia
	if resp == nil {
		clientUtils.Logger.Warn(fmt.Sprintf("Error de conexión al finalizar el proceso PID %d (respuesta nula)", procesoTernminado.PID))
	} else {
		clientUtils.Logger.Warn(fmt.Sprintf("Memoria rechazó la finalización del proceso PID %d. Status: %s", procesoTernminado.PID, resp.Status))
	}
	return false
}

// ------------ PLANIFICADOR CORTO PLAZO -----------------------------------------

type SchedulerEstrategy interface {
	selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo)
}

type FIFOScheduler struct {
}

func (f FIFOScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo) {
	proximo, ok := pcp.readyState.SacarProximoProceso()
	if ok {
		proximo.MT.readyTime += proximo.timeInState()
		pcp.ejecutar(proximo)
	}
}

// estos dos es checkpoint 3:
type SJFScheduler struct {
}

func (s SJFScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo)

type SJFDesScheduler struct {
}

func (sd SJFDesScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo)

type PlanificadorCortoPlazo struct {
	readyState         PCBList
	execState          PCBList
	schedulerEstrategy SchedulerEstrategy
}

func (pcp *PlanificadorCortoPlazo) RecibirProceso(proceso PCB) {
	proceso.timeInCurrentState = time.Now()
	pcp.readyState.Agregar(proceso)
}

func (pcp *PlanificadorCortoPlazo) ejecutar(proceso PCB) {
	CPUlibre, ok := ObtenerCpuLibre()
	if ok {
		// Log de cambio de estado READY -> EXEC
		clientUtils.Logger.Info(fmt.Sprintf(`Cambio de Estado: "## (%d) Pasa del estado READY al estado EXEC"`, proceso.PID))
		// Actualizamos el tiempo de entrada al estado EXEC
		proceso.timeInCurrentState = time.Now()
		proceso.ME.execCount++
		//Envío del proceso a CPU
		CPUlibre.PIDenEjecucion = proceso.PID
		CPUlibre.enviarProceso(proceso.PID, proceso.PC)
		pcp.execState.Agregar(proceso)
	} else {
		//TODO: ver solucion de volver a agregar a ready, si es fifo se agrega al final
		//Capaz en seleccionar proximo a ejecutar se podría guardar el proceso en una variable por ej en fifo pero no
		//eliminarlo de la lista ready, recien cuando se ejecuta y hay cpu libre, se lo saca de ready en ejecutar
		//Y
		clientUtils.Logger.Warn(fmt.Sprintf("No hay CPU libre para ejecutar el proceso PID %d", proceso.PID))
		pcp.readyState.Agregar(proceso) //Lo vuelvo a meter a la lista de ready
		// Pensar si mandarlo READY o implementar reintentos de planificación
	}

}

func (plp PlanificadorLargoPlazo) EnviarProcesoABlocked(proceso PCB, nombreIo string) {

	// Log del cambio de estado EXEC → BLOCKED
	clientUtils.Logger.Info(fmt.Sprintf(`Cambio de Estado: "## (%d) Pasa del estado EXEC al estado BLOCKED por IO %s"`, proceso.PID, nombreIo))

	//TODO: hacer metricas

	plp.pcp.execState.EliminarProcesoPorPID(proceso.PID)
	pmp.RecibirProceso(proceso)
}

// ------------ PLANIFICADOR MEDIANO PLAZO -----------------------------------------

var pmp PlanificadorMedianoPlazo

type PlanificadorMedianoPlazo struct {
	blockedState     PCBList
	suspBlockedState PCBList
	suspReadyState   PCBList
}

func (pmp *PlanificadorMedianoPlazo) RecibirProceso(proceso PCB) {
	//TODO: Ajustar metricas
	pmp.blockedState.Agregar(proceso)
}

//----------------------- Funciones para manejar los endpoints -------------------------

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

	cpusLibres = append(cpusLibres, nuevaCpu)
	clientUtils.Logger.Info(fmt.Sprintf("CPU registrada: %+v", nuevaCpu))
}

func BuscarCpuPorID(lista []Cpu, id string) (*Cpu, bool) {
	for i := range lista {
		if lista[i].Indentificador == id {
			return &lista[i], true
		}
	}
	return nil, false
}

// ResultadoProcesos es un endpoint placeholder para futuras devoluciones de la CPU
func ResultadoProcesos(w http.ResponseWriter, r *http.Request) {
	respuesta := serverUtils.RecibirPaquetes(w, r)
	cpuId := respuesta.Valores[0]
	cpu, ok := BuscarCpuPorID(cpusOcupadas, cpuId)
	if !ok {
		clientUtils.Logger.Info("Error al encontrar la cpu")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso, ok := Plp.pcp.execState.BuscarYSacarPorPID(cpu.PIDenEjecucion)
	if !ok {
		clientUtils.Logger.Info("Error al encontrar el proceso en ejecucion")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if respuesta.Valores[1] == "INIT_PROC" {
		tamProc, err := strconv.Atoi(respuesta.Valores[3])
		if err != nil {
			clientUtils.Logger.Info("Error al parsear tamaño de proceso")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		IniciarProceso(respuesta.Valores[2], uint(tamProc))
		//aca hay que mandar la cpu a que siga ejecutando

	} else if respuesta.Valores[1] == "EXIT" {
		Plp.FinalizarProceso(proceso)
		// aca hay que desocupar la cpu y ver que onda los procesos en ready
		// esto ahora lo vemos
		// seria algo asi
		//Plp.pcp.intentarEjecutar()
		// parecido al manejarLiberacion
	} else if respuesta.Valores[1] == "DUMP_MEMORY" {

	} else if respuesta.Valores[1] == "IO" {
		manejarIo(respuesta, proceso)
		//aca habria que desocupar la cpu porque el proceso
		// o se mnanda a blocked o se manda a exi
	} else {
		//TODO: un error de tipo syscall desconocida
	}

	w.WriteHeader(http.StatusOK)
}

func manejarIo(respuesta serverUtils.Paquete, proceso PCB) {
	nombre := respuesta.Valores[2]
	time, err := strconv.Atoi(respuesta.Valores[3])
	if err != nil {
		clientUtils.Logger.Info("Error al parsear el tiempo de interrupcion")
		return
	}
	io, ok := iosRegistradas[nombre]
	if ok {
		//mandar el proceso a bloqued
		// aca hay que crear el pmp( mediano plazo) es el que maneja blocked solo la estructura no lo tocamos por ahora
		if io.ocupada || !io.conectada {
			io.procesosEsperando = append(io.procesosEsperando, PedidoIo{PID: proceso.PID, time: time}) // agrego el PID
		} else {
			io.ocupada = true
			io.enviarProceso(proceso.PID, time)
		}

	} else {
		clientUtils.Logger.Error(fmt.Sprintf("Dispositivo %s no encontrado", nombre))
		Plp.FinalizarProceso(proceso)
	}
}

// RegistrarIo maneja el handshake de una IO
// Espera recibir un JSON con formato ["nombre", "ip", "puerto"]
func RegistrarIo(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)
	nombre := paquete.Valores[0]
	io, ok := iosRegistradas[nombre]
	if ok {
		io.conectada = true
		manejarPendientesIo(nombre)
	} else {
		puerto, err := strconv.Atoi(paquete.Valores[2])
		if err != nil {
			clientUtils.Logger.Info("Error al parsear puerto de IO")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nuevaIo := Io{
			Nombre:    paquete.Valores[0],
			Ip:        paquete.Valores[1],
			Puerto:    puerto,
			ocupada:   false,
			conectada: true,
		}

		iosRegistradas[nuevaIo.Nombre] = nuevaIo
		clientUtils.Logger.Info(fmt.Sprintf("IO registrada: %+v", nuevaIo))
	}

}

func ResultadoIos(w http.ResponseWriter, r *http.Request) {
	paquete := serverUtils.RecibirPaquetes(w, r)
	nombre := paquete.Valores[0]
	// TODO: aca habria que buscar el proceso
	// por el PID en blocked o en susp. blocked
	if paquete.Valores[1] == "Fin" {
		manejarPendientesIo(nombre)
	} else if paquete.Valores[1] == "Desconexion" {
		//TODO: mandar el proceso a exit
		manejarDesconexionIo(nombre)

	}
}

func manejarPendientesIo(nombre string) {
	io, ok := iosRegistradas[nombre]
	if !ok {
		clientUtils.Logger.Info("Error al buscar IO por nombre")
		return
	}
	if io.TieneProcesosEsperando() {
		pedido, ok := io.SacarProximoProceso()
		if !ok {
			clientUtils.Logger.Info("Error al obtener el proximo proceso de io")
			return
		}
		io.enviarProceso(pedido.PID, pedido.time)
	}
}

func manejarDesconexionIo(nombre string) {
	io, ok := iosRegistradas[nombre]
	if !ok {
		clientUtils.Logger.Info("Error al buscar IO por nombre")
		return
	}
	io.conectada = false
}

func IniciarProceso(filePath string, processSize uint) {
	nuevaPCB := PCB{PID: proximoPID, PC: 0, FilePath: filePath, ProcessSize: processSize}
	Plp.RecibirNuevoProceso(nuevaPCB)
	proximoPID++
}
