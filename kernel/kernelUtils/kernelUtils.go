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
var cpusRegistradas []Cpu
var iosRegistradas []Io

func BuscarCpuLibre() (Cpu, bool) {
	for _, cpu := range cpusRegistradas {
		if !cpu.Ocupada {
			return cpu, true
		}
	}
	var vacia Cpu
	return vacia, false
}

// PID para nuevos procesos
var proximoPID uint = 0
var Plp PlanificadorLargoPlazo

func IniciarConfiguracion(filePath string) *globalskernel.Config {
	var config *globalskernel.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

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
	Ocupada        bool
}

// TODO: Implementar el envio del PID
func (cpu Cpu) enviarProceso(PID uint)

type Io struct {
	Nombre string
	Ip     string
	Puerto int
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
	proceso.ME.newCount++
	proceso.MT.newTime += proceso.timeInState()
	plp.pcp.RecibirProceso(proceso)
}

func (plp *PlanificadorLargoPlazo) FinalizarProceso(proceso PCB) {
	proceso.timeInCurrentState = time.Now()
	proceso.ME.exitCount++
	if plp.EnviarFinalizacionMemoria(proceso) {
		// TODO: aca iria mediano plazo chequear los susps ready
		// si no chequea new
		plp.newAlgorithmEstrategy.manejarLiberacionDeProceso(plp)
		plp.loggearMetricas(proceso)
	}
	// TODO:
	// aca si sale mal iria un error
}

func (plp PlanificadorLargoPlazo) loggearMetricas(proceso PCB) {
	proceso.MT.exitTime += proceso.timeInState()
	// TODO: aca hay que simplemente loggear las metricas
}

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

// TODO: implementar el envio del aviso de finalizacion a memoria
func (plp PlanificadorLargoPlazo) EnviarFinalizacionMemoria(procesoTernminado PCB) bool

// ------------ PLANIFICADOR CORTO PLAZO -----------------------------------------

type SchedulerEstrategy interface {
	selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo)
}

type FIFOScheduler struct {
}

func (f FIFOScheduler) selecionarProximoAEjecutar(pcp *PlanificadorCortoPlazo) {
	proximo, ok := pcp.readyState.SacarProximoProceso()
	if ok {
		proximo.ME.readyCount++
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
	proceso.timeInCurrentState = time.Now()
	CPUlibre, ok := BuscarCpuLibre()
	if ok {
		CPUlibre.enviarProceso(proceso.PID)
	} else {
		//TODO: ver que hacer si no hay ninguna libre
	}

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
		Ocupada:        false,
	}

	cpusRegistradas = append(cpusRegistradas, nuevaCpu)
	clientUtils.Logger.Info(fmt.Sprintf("CPU registrada: %+v", nuevaCpu))
}

// ResultadoProcesos es un endpoint placeholder para futuras devoluciones de la CPU
func ResultadoProcesos(w http.ResponseWriter, r *http.Request) {
	clientUtils.Logger.Info("Recibido resultado de proceso (placeholder Checkpoint 1)")
	w.WriteHeader(http.StatusOK)
}

// RegistrarIo maneja el handshake de una IO
// Espera recibir un JSON con formato ["nombre", "ip", "puerto"]
func RegistrarIo(w http.ResponseWriter, r *http.Request) {

	paquete := serverUtils.RecibirPaquetes(w, r)

	puerto, err := strconv.Atoi(paquete.Valores[2])
	if err != nil {
		clientUtils.Logger.Info("Error al parsear puerto de IO")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	nuevaIo := Io{
		Nombre: paquete.Valores[0],
		Ip:     paquete.Valores[1],
		Puerto: puerto,
	}

	iosRegistradas = append(iosRegistradas, nuevaIo)
	clientUtils.Logger.Info(fmt.Sprintf("IO registrada: %+v", nuevaIo))
}

func IniciarProceso(filePath string, processSize uint) {
	nuevaPCB := PCB{PID: proximoPID, PC: 0, FilePath: filePath, ProcessSize: processSize}
	Plp.RecibirNuevoProceso(nuevaPCB)
	proximoPID++
}
