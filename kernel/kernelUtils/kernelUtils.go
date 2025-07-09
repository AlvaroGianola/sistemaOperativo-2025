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
	"sync/atomic"
	"time"

	globalskernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

// Listas globales para almacenar las CPUs e IOs conectadas

var cpusLibres CpuList
var cpusOcupadas CpuList
var sem_cpusLibres = make(chan int)
var iosRegistradas = IoMap{ios: make(map[string]*GrupoIo)}

// PID para nuevos procesos
var proximoPID uint = 0
var muProximoPID sync.Mutex
var Plp PlanificadorLargoPlazo
var Pmp PlanificadorMedianoPlazo

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

func IniciarPmp() PlanificadorMedianoPlazo {
	var estrategia SuspReadyAlgorithmEstrategy
	algoritmo := globalskernel.KernelConfig.ReadyIngressAlgorithm
	if algoritmo == "FIFO" {
		estrategia = SuspFIFOEstrategy{}
	} else if algoritmo == "PMCP" {
		estrategia = SuspPMCPEstrategy{}
	}
	return PlanificadorMedianoPlazo{suspReadyEstrategy: estrategia}
}

// Estructura para representar CPUs e IOs conectados al Kernel
type Cpu struct {
	Identificador            string `json:"identificador"`
	Ip                       string `json:"ip"`
	Puerto                   int    `json:"puerto"`
	PIDenEjecucion           uint
	sem_interrupcionAtendida chan struct{}
}

func (cpu *Cpu) enviarProceso(PID uint, PC uint) {
	valores := []string{strconv.Itoa(int(PID)), strconv.Itoa(int(PC))}
	paquete := clientUtils.Paquete{Valores: valores}
	cpu.PIDenEjecucion = PID
	//Mandamos el PID y PC al endpoint de CPU
	endpoint := "recibirProceso"

	clientUtils.EnviarPaquete(cpu.Ip, cpu.Puerto, endpoint, paquete)
}

func (cpu *Cpu) enviarInterrupcion(motivo string) {
	valores := []string{motivo}
	paquete := clientUtils.Paquete{Valores: valores}
	cpu.PIDenEjecucion = PID
	endpoint := "recibirInterrupcion"

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

func (cl *CpuList) SacarPorID(id string) *Cpu {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for i, cpu := range cl.cpus {
		if cpu.Identificador == id {
			// Eliminar del slice y retornar el puntero a la CPU removida
			cl.cpus = append(cl.cpus[:i], cl.cpus[i+1:]...)
			return &cpu
		}
	}
	// Si no se encuentra la CPU, retornar nil
	return nil
}

func (cl *CpuList) BuscarPorPIDEnEjecucion(pid uint) (*Cpu, bool) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for i := range cl.cpus {
		if cl.cpus[i].PIDenEjecucion == pid {
			return &cl.cpus[i], true
		}
	}
	return nil, false
}

func (cl *CpuList) Vacia() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return len(cl.cpus) == 0
}

// Struct y funciones para IO

type GrupoIo struct {
	Nombre            string
	Ios               []*Io
	procesosEsperando []PedidoIo
	mu                sync.Mutex
}

type Io struct {
	Ip             string
	Puerto         int
	ocupada        bool
	PIDEnEjecucion uint
	mu             sync.Mutex
}

type PedidoIo struct {
	PID  uint
	time int
}

func (gi *GrupoIo) TieneProcesosEsperando() bool {
	gi.mu.Lock()
	defer gi.mu.Unlock()
	return len(gi.procesosEsperando) > 0
}

func (gi *GrupoIo) ExistenInstancias() bool {
	gi.mu.Lock()
	defer gi.mu.Unlock()
	return len(gi.Ios) > 0
}

func (gi *GrupoIo) SacarProximoPedido() (PedidoIo, bool) {
	gi.mu.Lock()
	defer gi.mu.Unlock()
	if len(gi.procesosEsperando) == 0 {
		return PedidoIo{}, false
	}
	prox := gi.procesosEsperando[0]
	gi.procesosEsperando = gi.procesosEsperando[1:]
	return prox, true
}

func (gi *GrupoIo) AgregarPedido(p PedidoIo) {
	gi.mu.Lock()
	defer gi.mu.Unlock()
	gi.procesosEsperando = append(gi.procesosEsperando, p)
}

func (gi *GrupoIo) ObtenerIoLibre() (*Io, bool) {
	gi.mu.Lock()
	defer gi.mu.Unlock()
	for _, io := range gi.Ios {
		if !io.EstaOcupada() {
			io.MarcarOcupada()
			return io, true
		}
	}
	return nil, false
}

func (gi *GrupoIo) AgregarIo(io *Io) {
	gi.mu.Lock()
	defer gi.mu.Unlock()

	// Verifica que no esté ya incluida
	for _, existente := range gi.Ios {
		if existente.Ip == io.Ip && existente.Puerto == io.Puerto {
			return // Ya existe, no la agregamos
		}
	}
	gi.Ios = append(gi.Ios, io)
}

func (gi *GrupoIo) EliminarIo(io *Io) {
	gi.mu.Lock()
	defer gi.mu.Unlock()

	for i, existente := range gi.Ios {
		if existente.Ip == io.Ip && existente.Puerto == io.Puerto {
			// Eliminarla de la lista
			gi.Ios = append(gi.Ios[:i], gi.Ios[i+1:]...)
			break
		}
	}
}

func (gi *GrupoIo) BuscarIoPorPID(pid uint) (*Io, bool) {
	gi.mu.Lock()
	defer gi.mu.Unlock()

	for _, io := range gi.Ios {
		if io.ObtenerPIDEnEjecucion() == pid {
			return io, true
		}
	}
	return nil, false
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

func (io *Io) SetPIDEnEjecucion(pid uint) {
	io.mu.Lock()
	defer io.mu.Unlock()
	io.PIDEnEjecucion = pid
}

func (io *Io) ObtenerPIDEnEjecucion() uint {
	io.mu.Lock()
	defer io.mu.Unlock()
	return io.PIDEnEjecucion
}

func (io *Io) enviarProceso(PID uint, time int) {
	valores := []string{strconv.Itoa(int(PID)), strconv.Itoa(time)}
	paquete := clientUtils.Paquete{Valores: valores}
	io.SetPIDEnEjecucion(PID)
	//Mandamos el PID y tiempo al endpoint de IO
	endpoint := "recibirPeticion"

	clientUtils.EnviarPaquete(io.Ip, io.Puerto, endpoint, paquete)
}

type IoMap struct {
	ios map[string]*GrupoIo
	mu  sync.Mutex
}

// Obtener IO por nombre (retorna *Io para no copiar Mutex)
func (im *IoMap) ObtenerGrupo(nombre string) (*GrupoIo, bool) {
	im.mu.Lock()
	defer im.mu.Unlock()
	io, ok := im.ios[nombre]
	return io, ok
}

func (im *IoMap) AgregarGrupoIo(grupo *GrupoIo) {
	im.mu.Lock()
	defer im.mu.Unlock()

	if _, existe := im.ios[grupo.Nombre]; !existe {
		im.ios[grupo.Nombre] = grupo
	}
}

func (im *IoMap) BuscarIoPorGrupoYPID(nombre string, pid uint) (*Io, bool) {
	im.mu.Lock()
	grupo, ok := im.ios[nombre]
	im.mu.Unlock()
	if !ok {
		return nil, false
	}

	return grupo.BuscarIoPorPID(pid)
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
	PID                  uint
	PC                   uint
	ProcessSize          uint
	FilePath             string
	ME                   MetricasDeEstado
	MT                   MetricasDeTiempo
	timeInCurrentState   time.Time
	estimacion           float64
	estaSiendoDesalojado atomic.Bool
}

type PCBList struct {
	elementos []*PCB
	mu        sync.Mutex
}

func (p *PCBList) Agregar(proceso *PCB) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.elementos = append(p.elementos, proceso)
}

func (p *PCBList) SacarProximoProceso() (*PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.elementos) == 0 {
		var cero *PCB
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

func (p *PCBList) BuscarPorPID(pid uint) (*PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pcb := range p.elementos {
		if pcb.PID == pid {
			return pcb, true
		}
	}
	var cero *PCB
	return cero, false
}

func (p *PCBList) BuscarYSacarPorPID(pid uint) (*PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, pcb := range p.elementos {
		if pcb.PID == pid {
			p.elementos = append(p.elementos[:i], p.elementos[i+1:]...)
			return pcb, true
		}
	}
	var cero *PCB
	return cero, false
}

func (p *PCBList) Vacia() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.elementos) == 0
}

func (p *PCBList) VizualizarProximo() *PCB {
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

func (p *PCBList) SacarProcesoConMenorEstimacion() (*PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.elementos) == 0 {
		var cero *PCB
		return cero, false
	}

	minIndex := 0
	minValor := p.elementos[0].estimacion

	for i := 1; i < len(p.elementos); i++ {
		if p.elementos[i].estimacion < minValor {
			minIndex = i
			minValor = p.elementos[i].estimacion
		}
	}

	proceso := p.elementos[minIndex]
	p.elementos = append(p.elementos[:minIndex], p.elementos[minIndex+1:]...)

	return proceso, true
}

func (p *PCBList) BuscarProcesoConMayorEstimacion() (*PCB, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.elementos) == 0 {
		return nil, false
	}

	var maxProceso *PCB = p.elementos[0]
	for i := 1; i < len(p.elementos); i++ {
		if p.elementos[i].estaSiendoDesalojado.Load() {
			continue
		}
		if p.elementos[i].estimacion-p.elementos[i].timeInState() > maxProceso.estimacion-maxProceso.timeInState() {
			maxProceso = p.elementos[i]
		}
	}

	return maxProceso, true
}

func (p *PCB) timeInState() float64 {
	return float64(time.Since(p.timeInCurrentState).Microseconds()) / 1000.0
}

func (p *PCB) calcularProximaEstimacion(rafagaReal float64) {
	p.estimacion = globalskernel.KernelConfig.Alpha*rafagaReal + (1-globalskernel.KernelConfig.Alpha)*p.estimacion
}

//---------------- PLANIFICADOR LARGO PLAZO ---------------------------------------------

type NewAlgorithmEstrategy interface {
	manejarIngresoDeProceso(nuevoProceso *PCB, plp *PlanificadorLargoPlazo)
	manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo)
}

type FIFOEstrategy struct {
}

func (f FIFOEstrategy) manejarIngresoDeProceso(nuevoProceso *PCB, plp *PlanificadorLargoPlazo) {
	plp.newState.Agregar(nuevoProceso)
}

func (f FIFOEstrategy) manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo) {
	if Pmp.suspReadyState.Vacia() {
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
}

type PMCPEstrategy struct {
}

func (p PMCPEstrategy) manejarIngresoDeProceso(nuevoProceso *PCB, plp *PlanificadorLargoPlazo) {
	plp.newState.OrdenarPorPMC()
	procesoMasChico := plp.newState.VizualizarProximo()
	if Pmp.suspReadyState.Vacia() && nuevoProceso.ProcessSize < procesoMasChico.ProcessSize {
		plp.intentarInicializar(nuevoProceso)
	} else {
		plp.newState.Agregar(nuevoProceso)
	}
}

func (p PMCPEstrategy) manejarLiberacionDeProceso(plp *PlanificadorLargoPlazo) {
	if Pmp.suspReadyState.Vacia() {
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
	} else {
		Pmp.suspReadyState.OrdenarPorPMC()
		proximoProceso := Pmp.suspReadyState.VizualizarProximo()
		if Pmp.EnviarDesSuspensionPedidoMemoria(proximoProceso) {
			Pmp.EnviarProcesoAReady(proximoProceso)
			Pmp.suspReadyState.SacarProximoProceso()
			p.manejarLiberacionDeProceso(plp)
		}
	}
}

type PlanificadorLargoPlazo struct {
	newState              PCBList
	exitState             PCBList
	blockedState          PCBList
	newAlgorithmEstrategy NewAlgorithmEstrategy
	pcp                   PlanificadorCortoPlazo
}

func (plp *PlanificadorLargoPlazo) RecibirNuevoProceso(nuevoProceso *PCB) {
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Se crea el proceso - Estado: NEW", nuevoProceso.PID))
	nuevoProceso.timeInCurrentState = time.Now()
	if plp.newState.Vacia() && Pmp.suspReadyState.Vacia() {
		plp.intentarInicializar(nuevoProceso)
	} else {
		plp.newAlgorithmEstrategy.manejarIngresoDeProceso(nuevoProceso, plp)
	}
}

func (plp *PlanificadorLargoPlazo) blockedTimer(proceso *PCB) {
	timer := time.NewTimer(time.Duration(globalskernel.KernelConfig.SuspensionTime) * time.Millisecond)
	<-timer.C
	_, ok := plp.blockedState.BuscarYSacarPorPID(proceso.PID)
	if ok {
		proceso.MT.blockedTime += proceso.timeInState()
		Pmp.RecibirProcesoSuspblocked(proceso)
		plp.EnviarSuspensionMemoria(proceso)
		if Pmp.suspReadyState.Vacia() {
			plp.newAlgorithmEstrategy.manejarLiberacionDeProceso(plp)
		} else {
			Pmp.suspReadyEstrategy.manejarLiberacionDeProceso(&Pmp)
		}
	}
}

func (plp *PlanificadorLargoPlazo) RecibirProcesoBlocked(proceso *PCB) {
	proceso.timeInCurrentState = time.Now()
	proceso.ME.blockedCount++
	plp.blockedState.Agregar(proceso)
	go plp.blockedTimer(proceso)
}

func (plp *PlanificadorLargoPlazo) intentarInicializar(nuevoProceso *PCB) {
	if plp.EnviarPedidoMemoria(nuevoProceso) {
		plp.EnviarProcesoAReady(nuevoProceso)
	} else {
		plp.newState.Agregar(nuevoProceso)
	}
}

func (plp *PlanificadorLargoPlazo) EnviarProcesoAReady(proceso *PCB) {

	// Log del cambio de estado NEW → READY
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", proceso.PID))

	proceso.ME.newCount++
	proceso.MT.newTime += proceso.timeInState()

	proceso.timeInCurrentState = time.Now()
	plp.pcp.RecibirProceso(proceso)
}

func (plp *PlanificadorLargoPlazo) FinalizarProceso(proceso *PCB) {

	if plp.EnviarFinalizacionMemoria(proceso) {

		// Registramos el tiempo en el que el proceso entra en EXIT
		proceso.timeInCurrentState = time.Now()
		proceso.ME.exitCount++

		// Confirmamos la transición de EXEC → EXIT
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado EXEC al estado EXIT", proceso.PID))

		// Se registra en la lista de EXIT para registrar el cambio de estado
		plp.exitState.Agregar(proceso)
		plp.loggearMetricas(proceso)

		if Pmp.suspReadyState.Vacia() {
			plp.newAlgorithmEstrategy.manejarLiberacionDeProceso(plp)
		} else {
			Pmp.suspReadyEstrategy.manejarLiberacionDeProceso(&Pmp)
		}

	} else {
		// Logueamos el error si Memoria rechazó la finalización
		clientUtils.Logger.Error(fmt.Sprintf("Error: Memoria no aceptó finalizar el proceso PID %d", proceso.PID))
	}
}

func (plp *PlanificadorLargoPlazo) loggearMetricas(proceso *PCB) {
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
func (plp *PlanificadorLargoPlazo) EnviarPedidoMemoria(nuevoProceso *PCB) bool {

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

	if resp == nil {
		clientUtils.Logger.Warn(fmt.Sprintf("Error de conexión al tratar de inicializar el proceso PID %d (respuesta nula)", nuevoProceso.PID))
	} else {
		clientUtils.Logger.Warn(fmt.Sprintf("Memoria rechazó la iniciacion del proceso PID %d por espacio insuficiente. Status: %s", nuevoProceso.PID, resp.Status))
	}
	return false
}

// envio del aviso de finalizacion a memoria
func (plp *PlanificadorLargoPlazo) EnviarFinalizacionMemoria(procesoTernminado *PCB) bool {

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

func (plp *PlanificadorLargoPlazo) EnviarSuspensionMemoria(proceso *PCB) {
	valores := []string{strconv.Itoa(int(proceso.PID))}
	paquete := clientUtils.Paquete{Valores: valores}

	// Fijamos la direccion del endpoint de memoria
	ip := globalskernel.KernelConfig.IpMemory
	puerto := globalskernel.KernelConfig.PortMemory
	endpoint := "suspenderProceso"

	//Usamos EnviarPaqueteConRespuesta que devuelve la respuesta del servidor
	resp := clientUtils.EnviarPaqueteConRespuesta(ip, puerto, endpoint, paquete)
	if resp != nil && resp.StatusCode == http.StatusOK {
		clientUtils.Logger.Info(fmt.Sprintf("Memoria envió Proceso PID %d a swap correctamente", proceso.PID))

	}

	//Si no responde con 200 OK, lo logueamos como advertencia
	if resp == nil {
		clientUtils.Logger.Warn(fmt.Sprintf("Error de conexión al realizar swap del proceso PID %d (respuesta nula)", proceso.PID))
	} else {
		clientUtils.Logger.Warn(fmt.Sprintf("Memoria rechazó la solicitud de swap del proceso PID %d. Status: %s", proceso.PID, resp.Status))
	}
}

// ------------ PLANIFICADOR CORTO PLAZO -----------------------------------------

type SchedulerEstrategy interface {
	selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo)
	intentarDesalojo(pcp *PlanificadorCortoPlazo, proceso *PCB)
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

func (f FIFOScheduler) intentarDesalojo(pcp *PlanificadorCortoPlazo, proceso *PCB) {
}

type SJFScheduler struct {
}

func (s SJFScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo) {
	proximo, ok := pcp.readyState.SacarProcesoConMenorEstimacion()
	if ok {
		proximo.MT.readyTime += proximo.timeInState()
		pcp.ejecutar(proximo)
	}
}

func (s SJFScheduler) intentarDesalojo(pcp *PlanificadorCortoPlazo, proceso *PCB) {
}

type SRTScheduler struct {
}

func (s SRTScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo) {
	proximo, ok := pcp.readyState.SacarProcesoConMenorEstimacion()
	if ok {
		proximo.MT.readyTime += proximo.timeInState()
		pcp.ejecutar(proximo)
	}
}

func (s SRTScheduler) intentarDesalojo(pcp *PlanificadorCortoPlazo, procesoNuevo *PCB) {
	proceso, ok := pcp.execState.BuscarProcesoConMayorEstimacion()
	if !ok {
		clientUtils.Logger.Error("Error al buscar proceso con mayor estimación")
		return
	}
	if procesoNuevo.estimacion < (proceso.estimacion - proceso.timeInState()) {
		//buscar la cpu que tenga ese PID
		cpu, ok := cpusOcupadas.BuscarPorPIDEnEjecucion(proceso.PID)
		if !ok {
			clientUtils.Logger.Error(fmt.Sprintf("Error al encontrar CPU con proceso PID %d en ejecución", proceso.PID))
			return
		}
		proceso.estaSiendoDesalojado.Store(true)
		go cpu.enviarInterrupcion("DESALOJO")
		cpu.sem_interrupcionAtendida <- struct{}{}
		pcp.ejecutarConDesalojo(procesoNuevo, cpu)
	}
}

type PlanificadorCortoPlazo struct {
	readyState         PCBList
	execState          PCBList
	schedulerEstrategy SchedulerEstrategy
}

func (pcp *PlanificadorCortoPlazo) RecibirProceso(proceso *PCB) {
	proceso.timeInCurrentState = time.Now()
	proceso.ME.readyCount++
	pcp.readyState.Agregar(proceso)

	if cpusLibres.Vacia() {
		go pcp.schedulerEstrategy.intentarDesalojo(pcp, proceso)
	}

	sem_cpusLibres <- 0
	pcp.schedulerEstrategy.selecionarProximoAEjecutar(pcp)
}

func (pcp *PlanificadorCortoPlazo) ejecutar(proceso *PCB) {
	CPUlibre := cpusLibres.SacarProxima()
	// Log de cambio de estado READY -> EXEC
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado EXEC", proceso.PID))
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Ejecutando en CPU: %s", proceso.PID, CPUlibre.Identificador))
	// Actualizamos el tiempo de entrada al estado EXEC

	proceso.timeInCurrentState = time.Now()
	proceso.ME.execCount++
	pcp.execState.Agregar(proceso)
	CPUlibre.PIDenEjecucion = proceso.PID

	cpusOcupadas.Agregar(CPUlibre)
	CPUlibre.enviarProceso(proceso.PID, proceso.PC)
}

func (pcp *PlanificadorCortoPlazo) ejecutarConDesalojo(proceso *PCB, cpu *Cpu) {
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado EXEC", proceso.PID))
	proceso.timeInCurrentState = time.Now()
	proceso.ME.execCount++
	pcp.execState.Agregar(proceso)
	cpu.PIDenEjecucion = proceso.PID

	cpu.enviarProceso(proceso.PID, proceso.PC)
}

func (pcp *PlanificadorCortoPlazo) EnviarProcesoABlocked(proceso *PCB) {

	// Log del cambio de estado EXEC → BLOCKED
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado EXEC al estado BLOCKED", proceso.PID))

	Plp.RecibirProcesoBlocked(proceso)
}

// ------------ PLANIFICADOR MEDIANO PLAZO -----------------------------------------

type SuspReadyAlgorithmEstrategy interface {
	manejarIngresoDeProceso(proceso *PCB, pmp *PlanificadorMedianoPlazo)
	manejarLiberacionDeProceso(pmp *PlanificadorMedianoPlazo)
}

type SuspFIFOEstrategy struct {
}

func (f SuspFIFOEstrategy) manejarIngresoDeProceso(proceso *PCB, pmp *PlanificadorMedianoPlazo) {
	pmp.suspReadyState.Agregar(proceso)
}

func (f SuspFIFOEstrategy) manejarLiberacionDeProceso(pmp *PlanificadorMedianoPlazo) {

	proximoProceso := Pmp.suspReadyState.VizualizarProximo()
	if Pmp.EnviarDesSuspensionPedidoMemoria(proximoProceso) {
		Pmp.EnviarProcesoAReady(proximoProceso)
		Pmp.suspReadyState.SacarProximoProceso()
		f.manejarLiberacionDeProceso(pmp)
	}
}

type SuspPMCPEstrategy struct {
}

func (p SuspPMCPEstrategy) manejarIngresoDeProceso(proceso *PCB, pmp *PlanificadorMedianoPlazo) {
	pmp.suspReadyState.OrdenarPorPMC()
	procesoMasChico := pmp.suspReadyState.VizualizarProximo()
	if proceso.ProcessSize < procesoMasChico.ProcessSize {
		pmp.intentarInicializar(proceso)
	} else {
		pmp.suspReadyState.Agregar(proceso)
	}
}

func (p SuspPMCPEstrategy) manejarLiberacionDeProceso(pmp *PlanificadorMedianoPlazo) {
	Pmp.suspReadyState.OrdenarPorPMC()
	proximoProceso := Pmp.suspReadyState.VizualizarProximo()
	if Pmp.EnviarDesSuspensionPedidoMemoria(proximoProceso) {
		Pmp.EnviarProcesoAReady(proximoProceso)
		Pmp.suspReadyState.SacarProximoProceso()
		p.manejarLiberacionDeProceso(pmp)
	}
}

type PlanificadorMedianoPlazo struct {
	suspBlockedState   PCBList
	suspReadyState     PCBList
	suspReadyEstrategy SuspReadyAlgorithmEstrategy
}

func (pmp *PlanificadorMedianoPlazo) RecibirProcesoSuspblocked(proceso *PCB) {
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado SUSP BLOCKED", proceso.PID))
	proceso.timeInCurrentState = time.Now()
	proceso.ME.suspBlockedCount++
	pmp.suspBlockedState.Agregar(proceso)
}

func (pmp *PlanificadorMedianoPlazo) EnviarProcesoASuspReady(proceso *PCB) {
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado SUSP BLOCKED al estado SUSP READY", proceso.PID))
	proceso.timeInCurrentState = time.Now()
	proceso.ME.suspReadyCount++

	if Pmp.suspReadyState.Vacia() {
		pmp.intentarInicializar(proceso)
	} else {
		pmp.suspReadyEstrategy.manejarIngresoDeProceso(proceso, pmp)
	}
}

func (pmp *PlanificadorMedianoPlazo) intentarInicializar(proceso *PCB) {
	if pmp.EnviarDesSuspensionPedidoMemoria(proceso) {
		pmp.EnviarProcesoAReady(proceso)
	} else {
		pmp.suspReadyState.Agregar(proceso)
	}
}

func (pmp *PlanificadorMedianoPlazo) EnviarProcesoAReady(proceso *PCB) {
	clientUtils.Logger.Info(fmt.Sprintf("## (%d) Pasa del estado SUSP READY al estado READY", proceso.PID))
	proceso.MT.suspReadyTime += proceso.timeInState()
	proceso.timeInCurrentState = time.Now()
	Plp.pcp.RecibirProceso(proceso)
}

func (pmp *PlanificadorMedianoPlazo) EnviarDesSuspensionPedidoMemoria(proceso *PCB) bool {
	// Creamos el contenido del paquete con lo que la Memoria necesita:
	// PID, Ruta al pseudocódigo, y Tamaño del proceso
	valores := []string{
		strconv.Itoa(int(proceso.PID)),
		proceso.FilePath,
		strconv.Itoa(int(proceso.ProcessSize)),
	}

	// Construimos el paquete
	paquete := clientUtils.Paquete{Valores: valores}

	// Obtenemos IP y puerto de Memoria desde la config global del Kernel
	ip := globalskernel.KernelConfig.IpMemory
	puerto := globalskernel.KernelConfig.PortMemory
	endpoint := "desSuspenderProceso"

	resp := clientUtils.EnviarPaqueteConRespuesta(ip, puerto, endpoint, paquete)

	// Validamos la respuesta (por ahora asumimos éxito si hay respuesta 200 OK)
	if resp != nil && resp.StatusCode == http.StatusOK {
		clientUtils.Logger.Info(fmt.Sprintf("Proceso PID %d des-suspendido correctamente", proceso.PID))
		return true
	}

	clientUtils.Logger.Warn(fmt.Sprintf("Memoria rechazó el pedido de des-suspension del proceso PID %d", proceso.PID))
	return false
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
		Identificador:            paquete.Valores[0],
		Ip:                       paquete.Valores[1],
		Puerto:                   puerto,
		sem_interrupcionAtendida: make(chan struct{}),
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
	tiempoEjecucion := proceso.timeInState()
	proceso.MT.execTime += tiempoEjecucion
	proceso.calcularProximaEstimacion(tiempoEjecucion)
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

	if respuesta.Valores[MOTIVO_DEVOLUCION] == "INIT_PROC" {
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Solicitó syscall: INIT_PROC", proceso.PID))
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

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "EXIT" {
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Solicitó syscall: EXIT", proceso.PID))
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Estaba ejecutando en %s", proceso.PID, cpu.Identificador))
		Plp.FinalizarProceso(proceso)

		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Estoy por liberar CPU: %s", proceso.PID, cpu.Identificador))
		cpusLibres.Agregar(*cpusOcupadas.SacarPorID(cpu.Identificador))

		<-sem_cpusLibres

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "DUMP_MEMORY" {
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Solicitó syscall: DUMP_MEMORY", proceso.PID))
		go ManejarMemoryDump(proceso)
		cpusLibres.Agregar(*cpusOcupadas.SacarPorID(cpu.Identificador))

		<-sem_cpusLibres

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "IO" {
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Solicitó syscall: IO", proceso.PID))
		go manejarIo(respuesta, proceso)
		cpusLibres.Agregar(*cpusOcupadas.SacarPorID(cpu.Identificador))

		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Liberada CPU: %s", proceso.PID, cpu.Identificador))
		<-sem_cpusLibres

	} else if respuesta.Valores[MOTIVO_DEVOLUCION] == "DESALOJO" {
		clientUtils.Logger.Info(fmt.Sprintf("## (%d) - Desalojado por algoritmo SJF/SRT", proceso.PID))
		proceso.estaSiendoDesalojado.Store(false)
		Plp.pcp.RecibirProceso(proceso)
		<-cpu.sem_interrupcionAtendida
	} else {
		clientUtils.Logger.Error("Error, motivo de devolución de proceso desconocido")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func EnviarMemoryDump(PID uint) bool {
	valores := []string{
		strconv.Itoa(int(PID)),
	}

	// Construimos el paquete
	paquete := clientUtils.Paquete{Valores: valores}

	// Obtenemos IP y puerto de Memoria desde la config global del Kernel
	ip := globalskernel.KernelConfig.IpMemory
	puerto := globalskernel.KernelConfig.PortMemory
	endpoint := "memoryDump"

	resp := clientUtils.EnviarPaqueteConRespuesta(ip, puerto, endpoint, paquete)

	// Validamos la respuesta (por ahora asumimos éxito si hay respuesta 200 OK)
	if resp != nil && resp.StatusCode == http.StatusOK {
		clientUtils.Logger.Info(fmt.Sprintf("Proceso PID %d realizo correctamente un memory dump", PID))
		return true
	}

	clientUtils.Logger.Warn(fmt.Sprintf("Error al relizar el memory dump del proceso PID %d ", PID))
	return false
}

func ManejarMemoryDump(proceso *PCB) {
	Plp.pcp.EnviarProcesoABlocked(proceso)
	respuesta := EnviarMemoryDump(proceso.PID)
	proceso, ok := Plp.blockedState.BuscarYSacarPorPID(proceso.PID)
	proceso.MT.blockedTime += proceso.timeInState()
	if !ok {
		clientUtils.Logger.Error("Error al encontrar el proceso en blocked")
		return
	}
	if respuesta {
		Plp.pcp.RecibirProceso(proceso)
	} else {
		Plp.FinalizarProceso(proceso)
	}
}

func manejarIo(respuesta serverUtils.Paquete, proceso *PCB) {
	nombre := respuesta.Valores[NOMBRE_IO]
	time, err := strconv.Atoi(respuesta.Valores[TIME])
	if err != nil {
		clientUtils.Logger.Error("Error al parsear el tiempo de interrupcion")
		return
	}
	grupoIo, ok := iosRegistradas.ObtenerGrupo(nombre)
	if ok {
		clientUtils.Logger.Info(fmt.Sprintf(`## (%d) - Bloqueado por IO: %s`, proceso.PID, nombre))
		Plp.pcp.EnviarProcesoABlocked(proceso)
		ioDesocupada, ok := grupoIo.ObtenerIoLibre()
		if !ok {
			grupoIo.AgregarPedido(PedidoIo{PID: proceso.PID, time: time})
		} else {
			ioDesocupada.enviarProceso(proceso.PID, time)
		}
	} else {
		clientUtils.Logger.Error(fmt.Sprintf("No existe ninguna instancia del dispositivo %s", nombre))
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

	nuevaIo := &Io{
		Ip:      paquete.Valores[1],
		Puerto:  puerto,
		ocupada: false,
	}

	//manejo si se trata de una reconexion o una conexion nueva
	grupoIo, ok := iosRegistradas.ObtenerGrupo(nombre)
	if ok {
		grupoIo.AgregarIo(nuevaIo)
		manejarPendientesIo(nombre)
	} else {
		nuevoGrupo := &GrupoIo{Nombre: nombre}
		iosRegistradas.AgregarGrupoIo(nuevoGrupo)
		clientUtils.Logger.Info(fmt.Sprintf("IO registrada: %s", nombre))
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

	var estaEnBlocked bool
	proceso, estaEnSuspBlocked := Pmp.suspBlockedState.BuscarYSacarPorPID(uint(ioPid))
	if !estaEnSuspBlocked {
		proceso, estaEnBlocked = Plp.blockedState.BuscarYSacarPorPID(uint(ioPid))
		if !estaEnBlocked {
			clientUtils.Logger.Error("Error al encontrar el proceso en blocked")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}

	ioOcupada, ok := iosRegistradas.BuscarIoPorGrupoYPID(nombre, uint(ioPid))
	if !ok {
		clientUtils.Logger.Error("Error al buscar la IO por su PID")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if paquete.Valores[MOTIVO_DEVOLUCION_IO] == "Fin" {
		ioOcupada.MarcarLibre()
		go manejarPendientesIo(nombre)
		if estaEnBlocked {
			clientUtils.Logger.Info(fmt.Sprintf("## (%d) finalizó IO y pasa a READY", proceso.PID))
			proceso.MT.blockedTime += proceso.timeInState()
			Plp.pcp.RecibirProceso(proceso)
		} else if estaEnSuspBlocked {
			proceso.MT.suspBlockedTime += proceso.timeInState()
			Pmp.EnviarProcesoASuspReady(proceso)
		}
	} else if paquete.Valores[MOTIVO_DEVOLUCION_IO] == "Desconexion" {
		manejarDesconexionIo(nombre, ioOcupada)
		Plp.FinalizarProceso(proceso)
	}
}

func manejarPendientesIo(nombre string) {
	grupoIo, ok := iosRegistradas.ObtenerGrupo(nombre)
	if !ok {
		clientUtils.Logger.Error("Error al buscar una instancia IO por nombre")
		return
	}
	if grupoIo.TieneProcesosEsperando() {
		io, _ := grupoIo.ObtenerIoLibre()
		pedido, ok := grupoIo.SacarProximoPedido()
		if !ok {
			clientUtils.Logger.Error("Error al obtener el proximo proceso de io")
			return
		}
		io.enviarProceso(pedido.PID, pedido.time)
	}
}

func manejarDesconexionIo(nombre string, ioDesconectada *Io) {
	grupoIo, ok := iosRegistradas.ObtenerGrupo(nombre)
	if !ok {
		clientUtils.Logger.Error("Error al buscar el grupo IO por nombre")
		return
	}
	grupoIo.EliminarIo(ioDesconectada)
	if !grupoIo.ExistenInstancias() {
		finalizarTodosLosProcesosPendientes(grupoIo)
	}
}

func finalizarTodosLosProcesosPendientes(grupoIo *GrupoIo) {
	for grupoIo.TieneProcesosEsperando() {
		PedidoIo, ok := grupoIo.SacarProximoPedido()
		if !ok {
			break
		}
		var estaEnBlocked bool
		proceso, estaEnSuspBlocked := Pmp.suspBlockedState.BuscarYSacarPorPID(PedidoIo.PID)
		if estaEnSuspBlocked {
			proceso.MT.suspBlockedTime += proceso.timeInState()
		} else {
			proceso, estaEnBlocked = Plp.blockedState.BuscarYSacarPorPID(PedidoIo.PID)
			if !estaEnBlocked {
				clientUtils.Logger.Error("Error al encontrar el proceso en blocked")
				return
			}
			proceso.MT.blockedTime += proceso.timeInState()
		}
		Plp.FinalizarProceso(proceso)
	}
}

func IniciarKernel(filePath string, processSize uint) {
	fmt.Println("Presione ENTER para iniciar la planificación de Largo Plazo...")
	<-iniciarLargoPlazo
	IniciarProceso(filePath, processSize)
}

func IniciarProceso(filePath string, processSize uint) {
	muProximoPID.Lock()
	nuevaPCB := PCB{PID: proximoPID, PC: 0, FilePath: filePath, ProcessSize: processSize, estimacion: float64(globalskernel.KernelConfig.InitialEstimate)}
	proximoPID++
	muProximoPID.Unlock()

	Plp.RecibirNuevoProceso(&nuevaPCB)
}

func EsperarEnter() {
	for {
		bufio.NewReader(os.Stdin).ReadString('\n')
		iniciarLargoPlazo <- struct{}{}
	}
}
