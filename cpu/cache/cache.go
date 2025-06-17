package cache

import (
	"fmt"
	"strconv"
	"time"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func BuscarEnCache(pid int, pagina int) (string, bool) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	// Buscar en la caché
	for _, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			//CACHE DELAY
			time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))
			clientUtils.Logger.Info(fmt.Sprintf("Cache HIT - PID %d Página %d", pid, pagina))
			return entrada.Contenido, true
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
			globalsCpu.Cache[i].Contenido = contenido
			globalsCpu.Cache[i].Modificado = true
			clientUtils.Logger.Info(fmt.Sprintf("Cache Modify - PID %d Página %d", pid, pagina))
			return nil
		}
	}

	clientUtils.Logger.Error(fmt.Sprintf("No se encontró la entrada en caché para PID %d Página %d", pid, pagina))
	return fmt.Errorf("no se encontró la entrada en caché")
}

func AgregarACache(pid int, pagina int, dato string) {
	//CACHE DELAY
	time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))

	entrada := globalsCpu.EntradaCache{
		Pid:        pid,
		Pagina:     pagina,
		Contenido:  dato,
		Uso:        true,
		Modificado: false,
	}

	entrada.Contenido = dato

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
			evictada.Contenido,
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

func FlushPaginasModificadas(pid int) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()
	//1-Recorro las caches modificadas y con la tlb consigo su marco
	for _, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Modificado {
			marco, encontroMarco := tlbUtils.ConsultarMarco(entrada.Pagina)
			if !encontroMarco {
				clientUtils.Logger.Error(fmt.Sprintf("No se encontró el marco para la página %d del PID %d", entrada.Pagina, pid))
				continue
			}
			//2-Envio a memoria el contenido de la página
			valores := []string{
				strconv.Itoa(pid),
				strconv.Itoa(marco),
				entrada.Contenido,
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
