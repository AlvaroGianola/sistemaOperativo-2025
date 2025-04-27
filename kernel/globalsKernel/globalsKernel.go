package globalskernel

type Config struct {
	IpMemory              string `json:"ip_memory"`
	PortMemory            int    `json:"port_memory"`
	IpKernel              string `json:"ip_kernel"`
	PortKernel            int    `json:"port_kernel"`
	SchedulerAlgorithm    string `json:"scheduler_algorithm"`
	ReadyIngressAlgorithm string `json:"ready_ingress_algorithm"`
	Alpha                 string `json:"alpha"`
	SuspensionTime        int    `json:"suspension_time"`
	LogLevel              string `json:"log_level"`
}

var KernelConfig *Config

type Cola[T any] struct {
	elementos []T
}

// Encolar agrega un elemento al final de la cola
func (c *Cola[T]) Encolar(valor T) {
	c.elementos = append(c.elementos, valor)
}

// Desencolar saca el primer elemento de la cola
func (c *Cola[T]) Desencolar() (T, bool) {
	var cero T
	if len(c.elementos) == 0 {
		return cero, false
	}
	primero := c.elementos[0]
	c.elementos = c.elementos[1:]
	return primero, true
}

// Vacia dice si la cola está vacía
func (c *Cola[T]) Vacia() bool {
	return len(c.elementos) == 0
}

// Largo devuelve cuántos elementos hay
func (c *Cola[T]) Largo() int {
	return len(c.elementos)
}
