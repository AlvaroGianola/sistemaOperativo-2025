package globalsio

type Config struct {
	IPKernel   string `json:"ip_kernel"`
	PortKernel int    `json:"port_kernel"`
	IPIo       string `json:"ip_io"`
	PortIO     int    `json:"port_io"`
	LogLevel   string `json:"log_level"`
}

var IoConfig *Config
