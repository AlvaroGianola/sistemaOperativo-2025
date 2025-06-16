package cache

import (
	"fmt"
	"strconv"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	mmuUtils "github.com/sisoputnfrba/tp-golang/cpu/mmu"
	tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func BuscarEnCache(pid int, pagina int) (string, bool) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	direccionLogica := mmuUtils.ObtenerDireccionLogica(pagina)

	// Buscar en la caché
	for _, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			clientUtils.Logger.Info(fmt.Sprintf("Cache HIT - PID %d Página %d", pid, pagina))
			return entrada.Contenido[mmuUtils.ObtenerDesplazamiento(direccionLogica)], true
		}
	}

	clientUtils.Logger.Info(fmt.Sprintf("Cache MISS - PID %d Página %d", pid, pagina))
	return "", false
}

func ModificarContenidoCache(pid int, pagina int, contenido string) error {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	for i, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			globalsCpu.Cache[i].Contenido[mmuUtils.ObtenerDesplazamiento(mmuUtils.ObtenerDireccionLogica(pagina))] = contenido
			globalsCpu.Cache[i].Modificado = true
			clientUtils.Logger.Info(fmt.Sprintf("Cache Modify - PID %d Página %d", pid, pagina))
			return nil
		}
	}

	clientUtils.Logger.Error(fmt.Sprintf("No se encontró la entrada en caché para PID %d Página %d", pid, pagina))
	return fmt.Errorf("no se encontró la entrada en caché")
}

func LeerContenido(pid int, pagina int,tamanio int) (string, error) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	if globalsCpu.CpuConfig.CacheEntries > 0 {
		contenido,encontroContenido := BuscarEnCache(pid, pagina)

		if encontroContenido {
			clientUtils.Logger.Info(fmt.Sprintf("Cache HIT - PID %d Página %d", pid, pagina))
			return contenido[:tamanio], nil
		}

		clientUtils.Logger.Info(fmt.Sprintf("Cache MISS - PID %d Página %d", pid, pagina))
	}

	marco, err := mmuUtils.ObtenerMarco(pid, pagina)
	if err != nil {
		clientUtils.Logger.Error(fmt.Sprintf("Error al obtener marco para PID %d Página %d: %v", pid, pagina, err))
		return "", err
	}

	// Si no está en la caché, buscar en memoria
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(pagina),
		strconv.Itoa(marco),
	}
	paquete := clientUtils.Paquete{Valores: valores}
	respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paquete,
	)
	if respuesta == nil {
		clientUtils.Logger.Error(fmt.Sprintf("No se recibió respuesta de memoria para PID %d Página %d", pid, pagina))
		return "", fmt.Errorf("no se recibió respuesta de memoria")
	}
	contenido := string(respuesta)
	clientUtils.Logger.Info(fmt.Sprintf("Cache MISS - PID %d Página %d → Agregando a caché", pid, pagina))
	AgregarACache(pid, pagina, contenido)
	return contenido[:tamanio], nil

}

func AgregarACache(pid int, pagina int, dato string) {
	entrada := globalsCpu.EntradaCache{
		Pid:       pid,
		Pagina:    pagina,
		Contenido: []string{dato},
		Uso:       true,
		Modificado: false,
	}

	if len(globalsCpu.Cache) < globalsCpu.CpuConfig.CacheEntries {
		globalsCpu.Cache = append(globalsCpu.Cache, entrada)
		clientUtils.Logger.Info(fmt.Sprintf("Cache Add - PID %d Página %d", pid, pagina))
		return
	}

	for {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if globalsCpu.CpuConfig.CacheReplacment == "CLOCK" {
			if !actual.Uso {
				reemplazarEntradaCache(globalsCpu.PunteroClock, entrada)
				return
			} else {
				actual.Uso = false
			}
		} else if globalsCpu.CpuConfig.CacheReplacment == "CLOCK-M" {
			if !actual.Uso && !actual.Modificado {
				reemplazarEntradaCache(globalsCpu.PunteroClock, entrada)
				return
			}
			actual.Uso = false
		}
		globalsCpu.PunteroClock = (globalsCpu.PunteroClock + 1) % len(globalsCpu.Cache)
	}
}

func reemplazarEntradaCache(indice int, nueva globalsCpu.EntradaCache) {
	evictada := globalsCpu.Cache[indice]

	if evictada.Modificado {
		clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - Página %d modificada → escribir en Memoria", evictada.Pagina))

		valores := []string{
			strconv.Itoa(evictada.Pid),
			strconv.Itoa(evictada.Pagina),
			evictada.Contenido[mmuUtils.ObtenerDesplazamiento(mmuUtils.ObtenerDireccionLogica(evictada.Pagina))],
		}
		paquete := clientUtils.Paquete{Valores: valores}

		clientUtils.EnviarPaquete(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"writePagina",
			paquete,
		)
	}

	globalsCpu.Cache[indice] = nueva
	clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - PID %d Página %d → Nueva entrada", nueva.Pid, nueva.Pagina))
}

func FlushPaginasModificadas(pid int){
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()
	//1-Recorro las caches modificadas y con la tlb consigo su marco
	for _, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Modificado {
			marco,encontroMarco := tlbUtils.ConsultarMarco(entrada.Pagina)
			if !encontroMarco {
				clientUtils.Logger.Error(fmt.Sprintf("No se encontró el marco para la página %d del PID %d", entrada.Pagina, pid))
				continue
			}
			//2-Envio a memoria el contenido de la página
			valores := []string{
				strconv.Itoa(pid),
				strconv.Itoa(marco),
				entrada.Contenido[mmuUtils.ObtenerDesplazamiento(mmuUtils.ObtenerDireccionLogica(entrada.Pagina))],
			}
			paquete := clientUtils.Paquete{Valores: valores}
			clientUtils.EnviarPaquete(
				globalsCpu.CpuConfig.IpMemory,
				globalsCpu.CpuConfig.PortMemory,
				"writePagina",
				paquete,
			)

			clientUtils.Logger.Info(fmt.Sprintf("Cargando página %d del PID %d a la caché", entrada.Pagina, pid))
		}
	}
}

func LimpiarCache() {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	globalsCpu.Cache = []globalsCpu.EntradaCache{}
}