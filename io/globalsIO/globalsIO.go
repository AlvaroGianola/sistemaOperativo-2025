package globalsio

type Config struct {
	IPKernel     string `json:"ip_kernel"`
	PuertoKernel int    `json:"port_kernel"`
	PortIO       int    `json:"port_io"`
	LogLevel     string `json:"log_level"`
}

var IoConfig *Config