package mmu

import (
	"fmt"
	"time"
	"strconv"
	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
	tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
)

// MMU: Traduce dirección lógica a marco físico, usando TLB + Memoria
func ObtenerMarco(pid int, dirLogica int) (int, error) {
	pagina := dirLogica / globalsCpu.TamPagina

	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()

	// Buscar en TLB
	for i, entrada := range globalsCpu.Tlb {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			globalsCpu.Tlb[i].UltimoUso = time.Now()
			clientUtils.Logger.Info("TLB HIT", "PID", pid, "Página", pagina, "Marco", entrada.Marco)
			return entrada.Marco, nil
		}
	}

	clientUtils.Logger.Info("TLB MISS", "PID", pid, "Página", pagina)

	// Consultar a Memoria
	marco, err := consultarMarcoEnMemoria(pid, pagina)
	if err != nil {
		return -1, err
	}

	// Agregar a la TLB (manejo FIFO/LRU está adentro)
	tlbUtils.AgregarATLB(pid, pagina, marco)

	return marco, nil
}

func consultarMarcoEnMemoria(pid int, pagina int) (int, error) {
	valores := []string{strconv.Itoa(pid), strconv.Itoa(pagina)}
	paquete := clientUtils.Paquete{Valores: valores}

	resp := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"consultarMarco",
		paquete,
	)

	if resp == nil {
		return -1, fmt.Errorf("no se recibió respuesta de Memoria")
	}

	marco, err := strconv.Atoi(string(resp))
	if err != nil {
		return -1, fmt.Errorf("respuesta inválida al consultar marco")
	}

	return marco, nil
}