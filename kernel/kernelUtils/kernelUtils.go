package kernelUtils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	globalskernel "github.com/sisoputnfrba/tp-golang/kernel/globalsKernel"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	serverUtils "github.com/sisoputnfrba/tp-golang/utils/server"
)

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

type Io struct {
	Nombre string
	Ip     string
	Puerto int
}
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
	exitCount       float64
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

func (p *PCB) timeInState() float64 {
	return time.Since(p.timeInCurrentState).Seconds()
}

type NewAlgorithmEstrategy interface {
	proximoProceso(newQueue *globalskernel.Cola[PCB]) (PCB, bool)
	manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo)
}

type FIFOEstrategy struct {
}

func (f FIFOEstrategy) proximoProceso(newQueue *globalskernel.Cola[PCB]) (PCB, bool) {
	return newQueue.Desencolar()
}

func (f FIFOEstrategy) manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo) {
	plp.newQueue.Encolar(nuevoProceso)
}

type PMCPEstrategy struct {
}

func (p PMCPEstrategy) proximoProceso(newQueue *globalskernel.Cola[PCB]) (PCB, bool) {
	return PCB{}, false
}

func (p PMCPEstrategy) manejarIngresoDeProceso(nuevoProceso PCB, plp *PlanificadorLargoPlazo) {
}

type PlanificadorLargoPlazo struct {
	newQueue              globalskernel.Cola[PCB]
	exitQueue             globalskernel.Cola[PCB]
	newAlgorithmEstrategy NewAlgorithmEstrategy
	pcp                   PlanificadorCortoPlazo
}

func (plp *PlanificadorLargoPlazo) RecibirNuevoProceso(nuevoProceso PCB) {
	nuevoProceso.timeInCurrentState = time.Now()
	if plp.newQueue.Vacia() {
		plp.intentarInicializar(nuevoProceso)
	} else {
		plp.newAlgorithmEstrategy.manejarIngresoDeProceso(nuevoProceso, plp)
	}
}

func (plp PlanificadorLargoPlazo) intentarInicializar(nuevoProceso PCB) {
	if plp.EnviarPedidoMemoria(nuevoProceso) {
		plp.EnviarProcesoAReady(nuevoProceso)
	} else {
		plp.newQueue.Encolar(nuevoProceso)
	}
}

func (plp PlanificadorLargoPlazo) EnviarProcesoAReady(proceso PCB) {
	proceso.ME.newCount++
	proceso.MT.newTime += proceso.timeInState()
	plp.pcp.RecibirProceso(proceso)
}

// TODO: implementar el envio del proceso a memoria
func (plp PlanificadorLargoPlazo) EnviarPedidoMemoria(nuevoProceso PCB) bool

type PlanificadorCortoPlazo struct {
	readyQueue globalskernel.Cola[PCB]
}

func (pcp *PlanificadorCortoPlazo) RecibirProceso(proceso PCB) {
	proceso.timeInCurrentState = time.Now()
	pcp.readyQueue.Encolar(proceso)
}

// Listas globales para almacenar las CPUs e IOs conectadas
var cpusRegistradas []Cpu
var iosRegistradas []Io

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
}
