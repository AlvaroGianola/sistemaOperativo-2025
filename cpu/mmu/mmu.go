package mmu

import (
	"fmt"
	"math"
	"strconv"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func ObtenerDireccionLogica(nroPagina int) int {
	return nroPagina * globalsCpu.Memoria.TamanioPagina
}

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
	valores := []string{
		strconv.Itoa(pid),
	}

	for nivel := 1; nivel <= niveles; nivel++ {
		entrada := CalcularEntradaNivel(nroPagina, nivel, entradasPorTabla, niveles)
		valores = append(valores,strconv.Itoa(entrada))
	}
	
	paquete := clientUtils.Paquete{Valores: valores}

	respuesta := string(clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"accederMarcoUsuario",
		paquete,
	))

	marco,err := strconv.Atoi(respuesta)

	if err != nil {
		clientUtils.Logger.Error("Error al obtener marco de memoria", "error", err)
		return -1, fmt.Errorf("error al obtener marco de memoria: %w", err)
	}

	return marco,err

}

// MMU: Traduce dirección lógica a marco físico, usando TLB + Memoria
func ObtenerMarco(pid int, pagina int) (int, error) {
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()

	marco,encuentraMarco:= tlbUtils.ConsultarMarco(pagina) // Actualiza el último uso

	if encuentraMarco {
		clientUtils.Logger.Info("TLB HIT", "PID", pid, "Página", pagina, "Marco", marco)
		return marco, nil
	}

	clientUtils.Logger.Info("TLB MISS", "PID", pid, "Página", pagina)

	marco, err := ObtenerMarcoMultinivel(pid, pagina, globalsCpu.Memoria.NivelesPaginacion, globalsCpu.Memoria.CantidadEntradas)
	if err != nil {
		return -1, err
	}

	tlbUtils.AgregarATLB(pid, pagina, marco)
	return marco, nil
}
