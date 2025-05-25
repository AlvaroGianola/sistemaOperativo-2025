package kernelUtils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	globalskernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

// Listas globales para almacenar las CPUs e IOs conectadas

var cpusLibres CpuList
var cpusOcupadas CpuList
var iosRegistradas = IoMap{ios: make(map[string]*Io)}
var sem_cpusLibres = make(chan int)

// PID para nuevos procesos
var proximoPID uint = 0
var muProximoPID sync.Mutex
var Plp PlanificadorLargoPlazo

var iniciarLargoPlazo = make(chan struct{})

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
func InciarPcp() PlanificadorCortoPlazo {
	var estrategia SchedulerEstrategy
	algoritmo := globalskernel.KernelConfig.SchedulerAlgorithm
	if algoritmo == "FIFO" {
		estrategia = FIFOScheduler{}
	} else if algoritmo == "SFJ" {
		estrategia = SJFScheduler{}
	} else if algoritmo == "SRT" {
		estrategia = SRTScheduler{}
	}

	return PlanificadorCortoPlazo{schedulerEstrategy: estrategia}
}

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
	Identificador  string `json:"identificador"`
	Ip             string `json:"ip"`
	Puerto         int    `json:"puerto"`
	PIDenEjecucion uint
}

func (cpu *Cpu) enviarProceso(PID uint, PC uint) {
	valores := []string{strconv.Itoa(int(PID)), strconv.Itoa(int(PC))}
	paquete := clientUtils.Paquete{Valores: valores}
	cpu.PIDenEjecucion = PID
	//Mandamos el PID y PC al endpoint de CPU
	endpoint := "recibirProceso"

	clientUtils.EnviarPaquete(cpu.Ip, cpu.Puerto, endpoint, paquete)
}

type CpuList struct {
	cpus []Cpu
	mu   sync.Mutex
}

// Agregar una CPU
func (cl *CpuList) Agregar(cpu Cpu) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.cpus = append(cl.cpus, cpu)
}

// Sacar la primera CPU
func (cl *CpuList) SacarProxima() Cpu {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cpu := cl.cpus[0]
	cl.cpus = cl.cpus[1:]
	return cpu
}

// Buscar y sacar por ID
func (cl *CpuList) BuscarYSacarPorID(id string) (Cpu, bool) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for i, cpu := range cl.cpus {
		if cpu.Identificador == id {
			encontrada := cpu
			cl.cpus = append(cl.cpus[:i], cl.cpus[i+1:]...)
			return encontrada, true
		}
	}
	return Cpu{}, false
}

// Buscar por ID (sin eliminar)
func (cl *CpuList) BuscarPorID(id string) (*Cpu, bool) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for i := range cl.cpus {
		if cl.cpus[i].Identificador == id {
			return &cl.cpus[i], true
		}
	}
	return nil, false
}

func (cl *CpuList) SacarPorID(id string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for i, cpu := range cl.cpus {
		if cpu.Identificador == id {
			cl.cpus = append(cl.cpus[:i], cl.cpus[i+1:]...)
			break
		}
	}
}

func (cl *CpuList) Vacia() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return len(cl.cpus) == 0
}

// Struct y funciones para IO

type Io struct {
	Nombre            string
	Ip                string
	Puerto            int
	ocupada           bool
	conectada         bool
	procesosEsperando []PedidoIo
	mu                sync.Mutex
}

type PedidoIo struct {
	PID  uint
	time int
}

func (io *Io) TieneProcesosEsperando() bool {
	io.mu.Lock()
	defer io.mu.Unlock()
	return len(io.procesosEsperando) > 0
}

func (io *Io) SacarProximoProceso() (PedidoIo, bool) {
	io.mu.Lock()
	defer io.mu.Unlock()
	if len(io.procesosEsperando) == 0 {
		var vacio PedidoIo
		return vacio, false
	}
	prox := io.procesosEsperando[0]
	io.procesosEsperando = io.procesosEsperando[1:]
	return prox, true
}

func (io *Io) AgregarPedido(pedido PedidoIo) {
	io.mu.Lock()
	defer io.mu.Unlock()
	io.procesosEsperando = append(io.procesosEsperando, pedido)
}

func (io *Io) MarcarOcupada() {
	io.mu.Lock()
	defer io.mu.Unlock()
	io.ocupada = true
}

func (io *Io) MarcarLibre() {
	io.mu.Lock()
	defer io.mu.Unlock()
	io.ocupada = false
}

func (io *Io) EstaOcupada() bool {
	io.mu.Lock()
	defer io.mu.Unlock()
	return io.ocupada
}

func (io *Io) MarcarDesconectada() {
	io.mu.Lock()
	defer io.mu.Unlock()
	io.conectada = false
}

func (io *Io) MarcarConectada() {
	io.mu.Lock()
	defer io.mu.Unlock()
	io.conectada = true
}

func (io *Io) EstaConectada() bool {
	io.mu.Lock()
	defer io.mu.Unlock()
	return io.conectada
}

func (io *Io) enviarProceso(PID uint, time int) {
	valores := []string{strconv.Itoa(int(PID)), strconv.Itoa(time)}
	paquete := clientUtils.Paquete{Valores: valores}

	//Mandamos el PID y tiempo al endpoint de IO
	endpoint := "recibirPeticion"

	clientUtils.EnviarPaquete(io.Ip, io.Puerto, endpoint, paquete)
}

type IoMap struct {
	ios map[string]*Io
	mu  sync.Mutex
}

// Obtener IO por nombre (retorna *Io para no copiar Mutex)
func (im *IoMap) Obtener(nombre string) (*Io, bool) {
	im.mu.Lock()
	defer im.mu.Unlock()
	io, ok := im.ios[nombre]
	return io, ok
}

// Agregar o actualizar IO (usa puntero)
func (im *IoMap) Agregar(io *Io) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.ios[io.Nombre] = io
}

// Marcar desconectada (ya es un puntero, se modifica directamente)
func (im *IoMap) MarcarDesconectada(nombre string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if io, ok := im.ios[nombre]; ok {
		io.conectada = false
	}
}

//-------------- Estructuras generales para el manejo de estados -------------------

type MetricasDeEstado struct {
	newCount         uint
	readyCount       uint
	execCount        uint
	blockedCount     uint
	suspReadyCount   uint
	suspBlockedCount uint
	exitCount        uint
}

type MetricasDeTiempo struct {
	newTime         float64
	readyTime       float64
	execTime        float64
	blockedTime     float64
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
	mu        sync.Mutex
}

func (p *PCBList) Agregar(proceso PCB) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.elementos = append(p.elementos, proceso)
}

func (p *PCBList) SacarProximoProceso() (PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.elementos) == 0 {
		var cero PCB
		return cero, false
	}
	primero := p.elementos[0]
	p.elementos = p.elementos[1:]
	return primero, true
}

func (p *PCBList) EliminarProcesoPorPID(pid uint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, pcb := range p.elementos {
		if pcb.PID == pid {
			p.elementos = append(p.elementos[:i], p.elementos[i+1:]...)
			break
		}
	}
}

func (p *PCBList) BuscarPorPID(pid uint) (PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pcb := range p.elementos {
		if pcb.PID == pid {
			return pcb, true
		}
	}
	var cero PCB
	return cero, false
}

func (p *PCBList) BuscarYSacarPorPID(pid uint) (PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, pcb := range p.elementos {
		if pcb.PID == pid {
			p.elementos = append(p.elementos[:i], p.elementos[i+1:]...)
			return pcb, true
		}
	}
	var cero PCB
	return cero, false
}

func (p *PCBList) Vacia() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.elementos) == 0
}

func (p *PCBList) VizualizarProximo() PCB {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.elementos[0] //Si la lista esta está vacia puede dar panic
}

func (p *PCBList) OrdenarPorPMC() {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	if plp.newState.Vacia() {
		return
	}
	proximoProceso := plp.newState.VizualizarProximo()
	if plp.EnviarPedidoMemoria(proximoProceso) {
		plp.EnviarProcesoAReady(proximoProceso)
		plp.newState.SacarProximoProceso()
		f.manejarLiberacionDeProceso(plp)
	}
}

type PMCPEstrategy struct {
}

func (p PMCPEstrategy) manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo) {
	plp.intentarInicializar(nuevoProceso)
}

func (p PMCPEstrategy) manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo) {
	plp.newState.OrdenarPorPMC()
	if plp.newState.Vacia() {
		return
	}
	proximoProceso := plp.newState.VizualizarProximo()
	if plp.EnviarPedidoMemoria(proximoProceso) {
		plp.EnviarProcesoAReady(proximoProceso)
		plp.newState.SacarProximoProceso()
		p.manejarLiberacionDeProceso(plp)
	}
}

type PlanificadorLargoPlazo struct {
	newState              PCBList
	exitState             PCBList
	newAlgorithmEstrategy NewAlgorithmEstrategy
	pcp                   PlanificadorCortoPlazo
}

func (plp *PlanificadorLargoPlazo) RecibirNuevoProceso(nuevoProceso PCB) {
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Se crea el proceso - Estado: NEW", nuevoProceso.PID))
	nuevoProceso.timeInCurrentState = time.Now()
	if plp.newState.Vacia() {
		plp.intentarInicializar(nuevoProceso)
	} else {
		plp.newAlgorithmEstrategy.manejarIngresoDeProceso(nuevoProceso, plp)
	}
}

func (plp *PlanificadorLargoPlazo) intentarInicializar(nuevoProceso PCB) {
	if plp.EnviarPedidoMemoria(nuevoProceso) {
		plp.EnviarProcesoAReady(nuevoProceso)
	} else {
		plp.newState.Agregar(nuevoProceso)
	}
}

func (plp *PlanificadorLargoPlazo) EnviarProcesoAReady(proceso PCB) {

	// Log del cambio de estado NEW → READY
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", proceso.PID))

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
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado EXEC al estado EXIT", proceso.PID))

		// Se registra en la lista de EXIT para registrar el cambio de estado
		plp.exitState.Agregar(proceso)
		plp.loggearMetricas(proceso)

		// TODO: aca iria mediano plazo chequear los susps ready (Checkpoint 3)
		// si no chequea new
		if pmp.suspReadyState.Vacia() {
			plp.newAlgorithmEstrategy.manejarLiberacionDeProceso(plp)
		}

	} else {
		// Logueamos el error si Memoria rechazó la finalización
		clientUtils.Logger.Error(fmt.Sprintf("Error: Memoria no aceptó finalizar el proceso PID %d", proceso.PID))
	}
}

func (plp *PlanificadorLargoPlazo) loggearMetricas(proceso PCB) {
	proceso.MT.exitTime += proceso.timeInState()

	clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Finaliza el proceso", proceso.PID))

	clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Métricas de estado: NEW %d %.2f, READY %d %.2f, EXEC %d %.2f, BLOCKED  %d %.2f, SUSP_READY %d %.2f, SUSP_BLOCKED %d %.2f, EXIT %d %.2f",
		proceso.PID, proceso.ME.newCount, proceso.MT.newTime,
		proceso.ME.readyCount, proceso.MT.readyTime,
		proceso.ME.execCount, proceso.MT.execTime,
		proceso.ME.blockedCount, proceso.MT.blockedTime,
		proceso.ME.suspReadyCount, proceso.MT.suspReadyTime,
		proceso.ME.suspBlockedCount, proceso.MT.suspBlockedTime,
		proceso.ME.exitCount, proceso.MT.exitTime))
}

// pedido de inicialización de proceso devuelve si Memoria tiene espacio suficiente para inicializarlo
func (plp *PlanificadorLargoPlazo) EnviarPedidoMemoria(nuevoProceso PCB) bool {

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
func (plp *PlanificadorLargoPlazo) EnviarFinalizacionMemoria(procesoTernminado PCB) bool {

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
		clientUtils.Logger.Info(fmt.Sprintf("Memoria aceptó finalización de Proceso PID %d", procesoTernminado.PID))
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

func (s SJFScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo) {}

type SRTScheduler struct {
}

func (sd SRTScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo) {}

type PlanificadorCortoPlazo struct {
	readyState         PCBList
	execState          PCBList
	schedulerEstrategy SchedulerEstrategy
}

func (pcp *PlanificadorCortoPlazo) RecibirProceso(proceso PCB) {
	proceso.timeInCurrentState = time.Now()
	proceso.ME.readyCount++
	pcp.readyState.Agregar(proceso)

	sem_cpusLibres <- 0
	pcp.schedulerEstrategy.selecionarProximoAEjecutar(pcp)
}

func (pcp *PlanificadorCortoPlazo) ejecutar(proceso PCB) {
	CPUlibre := cpusLibres.SacarProxima()
	// Log de cambio de estado READY -> EXEC
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado EXEC", proceso.PID))
	// Actualizamos el tiempo de entrada al estado EXEC
	proceso.timeInCurrentState = time.Now()
	proceso.ME.execCount++
	pcp.execState.Agregar(proceso)
	CPUlibre.PIDenEjecucion = proceso.PID

	cpusOcupadas.Agregar(CPUlibre)
	CPUlibre.enviarProceso(proceso.PID, proceso.PC)
}

func (pcp *PlanificadorCortoPlazo) EnviarProcesoABlocked(proceso PCB, nombreIo string) {

	// Log del cambio de estado EXEC → BLOCKED
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado EXEC al estado BLOCKED", proceso.PID))
	clientUtils.Logger.Info(fmt.Sprintf(`## (%d) - Bloqueado por IO: %s`, proceso.PID, nombreIo))

	pmp.RecibirProceso(proceso)
}

// ------------ PLANIFICADOR MEDIANO PLAZO -----------------------------------------

var pmp PlanificadorMedianoPlazo

type PlanificadorMedianoPlazo struct {
	blockedState PCBList
	//suspBlockedState PCBList
	suspReadyState PCBList
}

func (pmp *PlanificadorMedianoPlazo) RecibirProceso(proceso PCB) {
	proceso.timeInCurrentState = time.Now()
	proceso.ME.blockedCount++
	pmp.blockedState.Agregar(proceso)
}

//----------------------- Funciones para manejar los endpoints -------------------------

// RegistrarCpu maneja el handshake de una CPU
// Espera recibir un JSON con formato ["ip", "puerto"]
func RegistrarCpu(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)

	puerto, err := strconv.Atoi(paquete.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear puerto de CPU")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	nuevaCpu := Cpu{
		Identificador: paquete.Valores[0],
		Ip:            paquete.Valores[1],
		Puerto:        puerto,
	}

	cpusLibres.Agregar(nuevaCpu)
	<-sem_cpusLibres
	clientUtils.Logger.Info(fmt.Sprintf("CPU registrada: %+v", nuevaCpu))

}

const (
	CPU_ID = iota
	PC
	MOTIVO_DEVOLUCION
	FILE_PATH
	TAM_PROC
	NOMBRE_IO = FILE_PATH
	TIME      = TAM_PROC
)

// ENDPOINT PARA LAS SYSCALLS
func ResultadoProcesos(w http.ResponseWriter, r *http.Request) {

	respuesta := serverUtils.RecibirPaquetes(w, r)
	cpuId := respuesta.Valores[CPU_ID]
	cpu, ok := cpusOcupadas.BuscarPorID(cpuId)
	if !ok {
		clientUtils.Logger.Error("Error al encontrar la cpu")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// saco el proceso de EXEC y acumulo cuanto tiempo estuvo ejecutando
	proceso, ok := Plp.pcp.execState.BuscarYSacarPorPID(cpu.PIDenEjecucion)
	proceso.MT.execTime += proceso.timeInState()
	if !ok {
		clientUtils.Logger.Error("Error al encontrar el proceso en ejecucion")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pcActualizado, err := strconv.Atoi(respuesta.Valores[PC])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PC del proceso")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	proceso.PC = uint(pcActualizado)

	//--------------------------- Manejo de las distintas syscalls ----------------------------------
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Solicitó syscall: %s", proceso.PID, respuesta.Valores[MOTIVO_DEVOLUCION]))

	if respuesta.Valores[MOTIVO_DEVOLUCION] == "INIT_PROC" {
		tamProc, err := strconv.Atoi(respuesta.Valores[TAM_PROC])
		if err != nil {
			clientUtils.Logger.Error("Error al parsear tamaño de proceso")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		proceso.timeInCurrentState = time.Now()
		Plp.pcp.execState.Agregar(proceso)
		go cpu.enviarProceso(proceso.PID, proceso.PC)

		go IniciarProceso(respuesta.Valores[FILE_PATH], uint(tamProc))
		//CPU sigue ejecutando

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "EXIT" {
		Plp.FinalizarProceso(proceso)
		cpusOcupadas.SacarPorID(cpu.Identificador)
		cpusLibres.Agregar(*cpu)
		<-sem_cpusLibres

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "DUMP_MEMORY" {

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "IO" {
		go manejarIo(respuesta, proceso)
		cpusOcupadas.SacarPorID(cpu.Identificador)
		cpusLibres.Agregar(*cpu)
		<-sem_cpusLibres
	} else {
		clientUtils.Logger.Error("Error, motivo de devolución de proceso desconocido")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func manejarIo(respuesta serverUtils.Paquete, proceso PCB) {
	nombre := respuesta.Valores[NOMBRE_IO]
	time, err := strconv.Atoi(respuesta.Valores[TIME])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear el tiempo de interrupcion")
		return
	}
	io, ok := iosRegistradas.Obtener(nombre)
	if ok {
		Plp.pcp.EnviarProcesoABlocked(proceso, nombre)
		if io.EstaOcupada() || !io.EstaConectada() {
			io.AgregarPedido(PedidoIo{PID: proceso.PID, time: time})
		} else {
			io.MarcarOcupada()
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
	puerto, err := strconv.Atoi(paquete.Valores[2])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear puerto de IO")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	//manejo si se trata de una reconexion o una conexion nueva
	io, ok := iosRegistradas.Obtener(nombre)
	if ok {
		io.MarcarConectada()
		io.Puerto = puerto // actualizo el puerto por las dudas nose si en esa nueva conexion el puerto viejo este ocupado por otra io
		manejarPendientesIo(nombre)
	} else {
		nuevaIo := &Io{
			Nombre:    paquete.Valores[0],
			Ip:        paquete.Valores[1],
			Puerto:    puerto,
			ocupada:   false,
			conectada: true,
		}

		iosRegistradas.Agregar(nuevaIo)
		clientUtils.Logger.Info(fmt.Sprintf("IO registrada: %+v", &nuevaIo))
	}
}

const (
	NOMBRE = iota
	MOTIVO_DEVOLUCION_IO
	PID
)

func ResultadoIos(w http.ResponseWriter, r *http.Request) {
	paquete := serverUtils.RecibirPaquetes(w, r)
	nombre := paquete.Valores[NOMBRE]
	ioPid, err := strconv.Atoi(paquete.Valores[PID])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear PID de IO")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	proceso, ok := pmp.blockedState.BuscarYSacarPorPID(uint(ioPid))
	proceso.MT.blockedTime += proceso.timeInState()
	if !ok {
		clientUtils.Logger.Error("Error al encontrar el proceso en blocked")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	//TODO:3er checkpoint enviar a susp ready en vez de ready
	if paquete.Valores[MOTIVO_DEVOLUCION_IO] == "Fin" {
		manejarPendientesIo(nombre)
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) finalizó IO y pasa a READY", proceso.PID))
		Plp.pcp.RecibirProceso(proceso)
	} else if paquete.Valores[MOTIVO_DEVOLUCION_IO] == "Desconexion" {
		manejarDesconexionIo(nombre)
		Plp.FinalizarProceso(proceso)
	}
}

func manejarPendientesIo(nombre string) {
	io, ok := iosRegistradas.Obtener(nombre)
	if !ok {
		clientUtils.Logger.Error("Error al buscar IO por nombre")
		return
	}
	if io.TieneProcesosEsperando() {
		pedido, ok := io.SacarProximoProceso()
		if !ok {
			clientUtils.Logger.Error("Error al obtener el proximo proceso de io")
			return
		}
		io.enviarProceso(pedido.PID, pedido.time)
	} else {
		io.MarcarLibre()
	}
}

func manejarDesconexionIo(nombre string) {
	io, ok := iosRegistradas.Obtener(nombre)
	if !ok {
		clientUtils.Logger.Error("Error al buscar IO por nombre")
		return
	}
	io.MarcarDesconectada()
}

func IniciarKernel(filePath string, processSize uint) {

	<-iniciarLargoPlazo
	IniciarProceso(filePath, processSize)
}

func IniciarProceso(filePath string, processSize uint) {
	muProximoPID.Lock()
	nuevaPCB := PCB{PID: proximoPID, PC: 0, FilePath: filePath, ProcessSize: processSize}
	proximoPID++
	muProximoPID.Unlock()

	Plp.RecibirNuevoProceso(nuevaPCB)
}

func EsperarEnter() {
	for {
		fmt.Println("Presione ENTER para iniciar la planificación de Largo Plazo...")
		bufio.NewReader(os.Stdin).ReadString('\n')
		iniciarLargoPlazo <- struct{}{}
	}
}
