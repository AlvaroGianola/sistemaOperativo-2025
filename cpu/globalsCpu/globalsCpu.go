package globalscpu

type Config struct {
	PortCpu         int    `json:"port_cpu"`
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