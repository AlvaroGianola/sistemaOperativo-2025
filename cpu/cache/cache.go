package cache

import (
	"fmt"
	"strconv"
	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)


// Accede a la caché de páginas: simula CLOCK o CLOCK-M según config
func AccederACache(pid int, pagina int, modificar bool) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	for i, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			globalsCpu.Cache[i].Uso = true
			if modificar {
				globalsCpu.Cache[i].Modificado = true
			}
			clientUtils.Logger.Info(fmt.Sprintf("Cache Hit - PID %d Página %d", pid, pagina))
			return
		}
	}

	clientUtils.Logger.Info(fmt.Sprintf("Cache Miss - PID %d Página %d", pid, pagina))
	AgregarACache(pid, pagina, modificar)
}

func AgregarACache(pid int, pagina int, modificar bool) {
	entrada := globalsCpu.EntradaCache{
		Pid:       pid,
		Pagina:    pagina,
		Marco:     pagina,
		Uso:       true,
		Modificado: modificar,
	}

	if len(globalsCpu.Cache) < globalsCpu.CpuConfig.CacheEntries {
		globalsCpu.Cache = append(globalsCpu.Cache, entrada)
		clientUtils.Logger.Info(fmt.Sprintf("Cache Add - PID %d Página %d", pid, pagina))
		return
	}

	// CLOCK o CLOCK-M
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
	eviectada := globalsCpu.Cache[indice]
	if eviectada.Modificado {
		clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - PID %d Página %d (dirty) → escribe en Memoria", eviectada.Pid, eviectada.Pagina))
		
		//Enviar página modificada a Memoria
		valores := []string{strconv.Itoa(eviectada.Pid), strconv.Itoa(eviectada.Pagina)}
		paquete := clientUtils.Paquete{Valores: valores}

		clientUtils.EnviarPaquete(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"escribirPaginaModificada",
			paquete,
		)
	}

	clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - PID %d Página %d", eviectada.Pid, eviectada.Pagina))
	globalsCpu.Cache[indice] = nueva
	clientUtils.Logger.Info(fmt.Sprintf("Cache Add - PID %d Página %d", nueva.Pid, nueva.Pagina))
}
