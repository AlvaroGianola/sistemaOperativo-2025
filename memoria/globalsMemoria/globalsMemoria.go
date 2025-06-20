package globalsmemoria

import (
	"sync"
)

type Config struct {
	PortMemory     int    `json:"port_memory"`
	IpMemory       string `json:"ip_memory"`
	MemorySize     int    `json:"memory_size"`
	PageSize       int    `json:"page_size"`
	EntriesPerPage int    `json:"entries_per_page"`
	NumberOfLevels int    `json:"number_of_levels"`
	MemoryDelay    int    `json:"memory_delay"`
	SwapfilePath   string `json:"swapfile_path"`
	SwapDelay      int    `json:"swap_delay"`
	LogLevel       string `json:"log_level"`
	DumpPath       string `json:"dump_path"`
	ScriptsPath    string `json:"scripts_path"`
}

var MemoriaConfig *Config

type Proceso struct {
	Pid                int
	Pc                 int
	TablaPaginasGlobal TablaPaginas
	Size               int
	Metricas           MetricasProceso
}

type MetricasProceso struct {
	AccesosATablas           int
	InstruccionesSolicitadas int
	BajadasASwap             int
	SubidasAMemoria          int
	LecturasDeMemoria        int
	EscriturasDeMemoria      int
}

func NewMetricasProceso() MetricasProceso {
	return MetricasProceso{
		AccesosATablas:           0,
		InstruccionesSolicitadas: 0,
		BajadasASwap:             0,
		SubidasAMemoria:          0,
		LecturasDeMemoria:        0,
		EscriturasDeMemoria:      0,
	}
}

type EntradaTabla interface {
	EsPagina() bool
}

type Pagina struct {
	Marco         int
	Validez       bool
	BitUso        bool
	BitModificado bool
	Permisos      struct {
		Escritura bool
		Lectura   bool
	}
	MutexPagina sync.Mutex
}

func NewPagina(marco int, validez bool, escritura bool, lectura bool) Pagina {
	return Pagina{
		Marco:         marco,
		Validez:       validez,
		BitUso:        false,
		BitModificado: false,
		Permisos: struct {
			Escritura bool
			Lectura   bool
		}{
			Escritura: escritura,
			Lectura:   lectura,
		},
		MutexPagina: sync.Mutex{},
	}
}

func (p *Pagina) EsPagina() bool {
	return true
}

type TablaPaginas struct {
	Entradas    []EntradaTabla // índice -> tabla o página
	Nivel       int
	MutexPagina sync.Mutex
}

func NewTablaPaginas(nivel int) TablaPaginas {
	return TablaPaginas{
		Entradas:    make([]EntradaTabla, MemoriaConfig.EntriesPerPage),
		Nivel:       nivel,
		MutexPagina: sync.Mutex{},
	}
}

func (t *TablaPaginas) EsPagina() bool {
	return false
}

var MemoriaUsuario = make([]byte, MemoriaConfig.MemorySize)

var BitmapMarcosLibres []bool = make([]bool, MemoriaConfig.MemorySize/MemoriaConfig.PageSize)

var ProcesosEnMemoria []Proceso

var MutexProcesos sync.Mutex
var MutexBitmapMarcosLibres sync.Mutex
