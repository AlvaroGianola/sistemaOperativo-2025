package memoriaUtils

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type Config struct {
	PortMemory     int    `json:"port_memory"`
	MemorySize     int    `json:"memory_size"`
	PageSize       int    `json:"page_size"`
	EntriesPerPage int    `json:"entries_per_page"`
	NumberOfLevels int    `json:"number_of_levels"`
	MemoryDelay    int    `json:"memory_delay"`
	SwapfilePath   string `json:"swapfile_path"`
	SwapDelay      int    `json:"swap_delay"`
	LogLevel       string `json:"log_level"`
	DumpPath       string `json:"dump_path"`
}

var MemoriaConfig *Config

func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

func RecibirPeticionCpu(w http.ResponseWriter, r *http.Request)

func RecibirPeticionKernel(w http.ResponseWriter, r *http.Request)
