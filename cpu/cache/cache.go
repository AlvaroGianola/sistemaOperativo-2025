package cache

import (
	"fmt"
	"strconv"
	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)


// Accede a la caché de páginas: simula CLOCK o CLOCK-M según config
func AccederACache(pid int, pagina int, modificar bool, contenido string) string {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	// Buscar en la caché
	for i, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			globalsCpu.Cache[i].Uso = true
			if modificar {
				globalsCpu.Cache[i].Modificado = true
				globalsCpu.Cache[i].Contenido = contenido
			}
			clientUtils.Logger.Info(fmt.Sprintf("Cache HIT - PID %d Página %d", pid, pagina))
			return globalsCpu.Cache[i].Contenido
		}
	}

	clientUtils.Logger.Info(fmt.Sprintf("Cache MISS - PID %d Página %d", pid, pagina))
	AgregarACache(pid, pagina, contenido, modificar)
	return contenido
}


func AgregarACache(pid int, pagina int, contenido string, modificar bool) {
	entrada := globalsCpu.EntradaCache{
		Pid:       pid,
		Pagina:    pagina,
		Contenido: contenido,
		Uso:       true,
		Modificado: modificar,
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

		// Simulá envío a Memoria si querés (opcional)
		valores := []string{
			strconv.Itoa(evictada.Pid),
			strconv.Itoa(evictada.Pagina),
			evictada.Contenido,
		}
		paquete := clientUtils.Paquete{Valores: valores}

		clientUtils.EnviarPaquete(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"writePaginaModificada",
			paquete,
		)
	}

	globalsCpu.Cache[indice] = nueva
	clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - PID %d Página %d → Nueva entrada", nueva.Pid, nueva.Pagina))
}
