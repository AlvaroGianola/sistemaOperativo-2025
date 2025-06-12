package mmu

import (
	"fmt"
	"time"
	"strconv"
	"math"

globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
)

func ObtenerNumeroDePagina(direccionLogica int) int {
	return int(math.Floor(float64(direccionLogica) / float64(globalsCpu.Memoria.TamanioPagina)))
}

func ObtenerDesplazamiento(direccionLogica int) int {
	return direccionLogica % globalsCpu.Memoria.TamanioPagina
}

func CalcularEntradaNivel(nroPagina, nivel, cantEntradas, niveles int) int {
	exponente := niveles - nivel
	divisor := int(math.Pow(float64(cantEntradas), float64(exponente)))
	return (nroPagina / divisor) % cantEntradas
}

func ObtenerMarcoMultinivel(pid int, direccionLogica int, niveles int, entradasPorTabla int) (int, error) {
	nroPagina := ObtenerNumeroDePagina(direccionLogica)
	idTablaActual := 0 // raíz

	for nivel := 1; nivel <= niveles; nivel++ {
		entrada := CalcularEntradaNivel(nroPagina, nivel, entradasPorTabla, niveles)

		valores := []string{
			strconv.Itoa(pid),
			strconv.Itoa(idTablaActual),
			strconv.Itoa(nivel),
			strconv.Itoa(entrada),
		}
		paquete := clientUtils.Paquete{Valores: valores}

		respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"obtenerEntradaTabla",
			paquete,
		)

		if respuesta == nil {
			return -1, fmt.Errorf("no se recibió respuesta en nivel %d", nivel)
		}

		id, err := strconv.Atoi(string(respuesta))
		if err != nil {
			return -1, fmt.Errorf("respuesta inválida en nivel %d", nivel)
		}

		idTablaActual = id
	}

	return idTablaActual, nil // marco en último nivel
}

// MMU: Traduce dirección lógica a marco físico, usando TLB + Memoria
func ObtenerMarco(pid int, pagina int) (int, error) {
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()

	for i, entrada := range globalsCpu.Tlb {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			globalsCpu.Tlb[i].UltimoUso = time.Now()
			clientUtils.Logger.Info("TLB HIT", "PID", pid, "Página", pagina, "Marco", entrada.Marco)
			return entrada.Marco, nil
		}
	}

	clientUtils.Logger.Info("TLB MISS", "PID", pid, "Página", pagina)

	marco, err := ObtenerMarcoMultinivel(pid, pagina, globalsCpu.Memoria.NivelesPaginacion, globalsCpu.Memoria.CantidadEntradas)
	if err != nil {
		return -1, err
	}

	tlbUtils.AgregarATLB(pid, pagina, marco)
	return marco, nil
}
