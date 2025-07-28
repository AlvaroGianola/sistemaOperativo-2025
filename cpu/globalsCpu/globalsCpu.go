package globalscpu

import (
	"sync"
	"time"
)

type Config struct {
	PortCpu         int    `json:"port_cpu"`
	IpCpu           string `json:"ip_cpu"`
	IpMemory        string `json:"ip_memory"`
	PortMemory      int    `json:"port_memory"`
	IpKernel        string `json:"ip_kernel"`
	PortKernel      int    `json:"port_kernel"`
	TlbEntries      int    `json:"tlb_entries"`
	TlbReplacement  string `json:"tlb_replacement"`
	CacheEntries    int    `json:"cache_entries"`
	CacheReplacment string `json:"cache_replacement"`
	CacheDelay      int    `json:"cache_delay"`
	LogLevel        string `json:"log_level"`
}

// Representa un proceso con su PID y su Program Counter (PC)
type Proceso struct {
	Pid int `json:"pid"`
	Pc  int `json:"pc"`
}

type CaracteristicasMemoria struct {
	TamanioPagina     int
	NivelesPaginacion int
	CantidadEntradas  int
}

type EntradaTLB struct {
	Pid             int
	Pagina          int
	Marco           int
	UltimoUso       time.Time
	InstanteCargado time.Time
}

type EntradaCache struct {
	Pid        int
	Pagina     int
	Contenido  []byte
	Uso        bool
	Modificado bool
	Offset     int
}

type Interrupcion struct {
	ExisteInterrupcion bool
	Motivo             string
}

var (
	Tlb      []EntradaTLB
	TlbMutex sync.Mutex

	Cache        []EntradaCache
	CacheMutex   sync.Mutex
	PunteroClock int
)

var Interrupciones Interrupcion = Interrupcion{}

var CpuConfig *Config

var Memoria CaracteristicasMemoria

var Identificador string

func SetIdentificador(nuevoId string) {
	Identificador = nuevoId
}

//Mocks de instrucciones de un proceso

func ObtenerMix(pc int, pid int) string {
	switch pc {
	case 0:
		return "NOOP"
	case 1:
		return "READ 0 2"
	case 2:
		return "EXIT"
	default:
		return "NOOP"
	}
}

func ObtenerInstruccion(pc int, pid int) string {
	switch pc {
	case 0:
		return "NOOP"
	case 1:
		return "WRITE 0 HOLA_CPU"
	case 2:
		return "READ 0 2"
	case 3:
		return "GOTO 1"
	case 4:
		return "EXIT"
	default:
		return "NOOP"
	}
}

func ObtenerSyscall(pc int, pid int) string {
	switch pc {
	case 0:
		return "IO IMPRESORA 25000"
	case 1:
		return "INIT_PROC proceso1 256"
	case 2:
		return "DUMP_MEMORY"
	case 3:
		return "EXIT"
	default:
		return "NOOP"
	}
}
