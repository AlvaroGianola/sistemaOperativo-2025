package globalscpu

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

var CpuConfig *Config

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
		return "READ resources/test.txt 2"
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
		return "WRITE resources/test.txt HOLA_CPU"
	case 2:
		return "READ resources/test.txt 2"
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
