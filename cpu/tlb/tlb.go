package tlb

import (
	"time"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func AgregarATLB(pid int, pagina int, marco int) {
	entrada := globalsCpu.EntradaTLB{
		Pid:             pid,
		Pagina:          pagina,
		Marco:           marco,
		UltimoUso:       time.Now(),
		InstanteCargado: time.Now(),
	}

	if len(globalsCpu.Tlb) < globalsCpu.CpuConfig.TlbEntries {
		globalsCpu.Tlb = append(globalsCpu.Tlb, entrada)
		clientUtils.Logger.Info("TLB Add", "PID", pid, "Página", pagina, "Marco", marco)
		return
	}

	// Reemplazo por FIFO o LRU
	switch globalsCpu.CpuConfig.TlbReplacement {
	case "FIFO":
		indice := BuscarEntradaMasVieja()
		clientUtils.Logger.Debug("Se reemplazo la tlb: ", "TLB: ", globalsCpu.Tlb[indice])
		globalsCpu.Tlb[indice] = entrada
	case "LRU":
		indice := BuscarEntradaMenosUsada()
		clientUtils.Logger.Debug("Se reemplazo la tlb: ", "TLB: ", globalsCpu.Tlb[indice])
		globalsCpu.Tlb[indice] = entrada
	}
	clientUtils.Logger.Info("TLB Replace", "PID", pid, "Página", pagina, "Marco", marco)
}

func BuscarEntradaMasVieja() int {
	masVieja := 0
	for i := 1; i < len(globalsCpu.Tlb); i++ {
		if globalsCpu.Tlb[i].InstanteCargado.Before(globalsCpu.Tlb[masVieja].InstanteCargado) {
			masVieja = i
		}
	}
	return masVieja
}

func BuscarEntradaMenosUsada() int {
	menosUsada := 0
	for i := 1; i < len(globalsCpu.Tlb); i++ {
		if globalsCpu.Tlb[i].UltimoUso.Before(globalsCpu.Tlb[menosUsada].UltimoUso) {
			menosUsada = i
		}
	}
	return menosUsada
}

func ConsultarMarco(pagina int) (int, bool) {
	if globalsCpu.CpuConfig.TlbEntries == 0 {
		return -1, false
	}

	for i, entrada := range globalsCpu.Tlb {
		if entrada.Pagina == pagina {
			globalsCpu.Tlb[i].UltimoUso = time.Now()
			return entrada.Marco, true
		}
	}
	return -1, false
}

func LimpiarTLB() {
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()
	if globalsCpu.CpuConfig.TlbEntries != 0 {
		globalsCpu.Tlb = []globalsCpu.EntradaTLB{}
		clientUtils.Logger.Info("TLB Cleared")
	}
}
