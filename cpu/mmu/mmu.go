package mmu

import (
	"fmt"
	"math"
	"strconv"
	"time"

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
	return int(math.Floor(float64(nroPagina)/float64(divisor))) % cantEntradas
}

func ObtenerMarcoMultinivel(pid int, direccionLogica int, niveles int, entradasPorTabla int) (int, error) {
	nroPagina := ObtenerNumeroDePagina(direccionLogica)
	valores := []string{
		strconv.Itoa(pid),
	}

	for nivel := 1; nivel <= niveles; nivel++ {
		entrada := CalcularEntradaNivel(nroPagina, nivel, entradasPorTabla, niveles)
		valores = append(valores, strconv.Itoa(entrada))
		time.Sleep(1000)
	}

	desplazamiento := strconv.Itoa(ObtenerDesplazamiento(direccionLogica))

	valores = append(valores, desplazamiento)

	paquete := clientUtils.Paquete{Valores: valores}

	//clientUtils.Logger.Debug("Paquete a enviar en accederMarcoUsuario", "paquete", paquete)

	resBytes := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"accederMarcoUsuario",
		paquete,
	)

	if resBytes == nil {
		clientUtils.Logger.Error("Error: no se recibió respuesta de Memoria (accederMarcoUsuario)")
		return -1, fmt.Errorf("no se recibió respuesta de Memoria")
	}

	respuesta := string(resBytes)
	marco, err := strconv.Atoi(respuesta)

	if err != nil {
		clientUtils.Logger.Error("Error al convertir marco de memoria", "respuesta", respuesta, "error", err)
		return -1, fmt.Errorf("error al convertir marco: %w", err)
	}

	return marco, nil

}

// MMU: Traduce dirección lógica a marco físico, usando TLB + Memoria
func ObtenerMarco(pid int, direccionLogica int) (int, error) {
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()
	pagina := ObtenerNumeroDePagina(direccionLogica)

	marco, encuentraMarco := tlbUtils.ConsultarMarco(pagina) // Actualiza el último uso

	if encuentraMarco {
		clientUtils.Logger.Info(fmt.Sprintf("PID: %d - TLB HIT - Pagina: %d", pid, pagina))
		return marco, nil
	}

	if globalsCpu.CpuConfig.TlbEntries != 0 {
		clientUtils.Logger.Info(fmt.Sprintf("PID: %d - TLB MISS - Pagina: %d", pid, pagina))
	}

	marco, err := ObtenerMarcoMultinivel(pid, direccionLogica, globalsCpu.Memoria.NivelesPaginacion, globalsCpu.Memoria.CantidadEntradas)
	if err != nil {
		return -1, err
	}

	if globalsCpu.CpuConfig.TlbEntries != 0 {
		tlbUtils.AgregarATLB(pid, pagina, marco)
	}

	return marco, nil
}
