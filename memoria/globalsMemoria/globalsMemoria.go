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
	ScriptsPath string `json:"scripts_path"`
}

var MemoriaConfig *Config

type Proceso struct {
	Instrucciones []string
	Size          int
}

var MutexProcesos sync.Mutex
