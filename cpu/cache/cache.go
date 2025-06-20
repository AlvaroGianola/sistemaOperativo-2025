package cache

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	mmuUtils "github.com/sisoputnfrba/tp-golang/cpu/mmu"
	tlbUtils "github.com/sisoputnfrba/tp-golang/cpu/tlb"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func BuscarPaginaEnCache(pid int, pagina int) ([]byte, bool) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	// Buscar en la caché
	for _, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			//CACHE DELAY
			time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))
			clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache HIT - Pagina: %d", pid, pagina))
			return entrada.Contenido, true
		}
	}

	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache MISS - Pagina: %d", pid, pagina))
	return nil, false
}

func ModificarContenidoCache(pid int, pagina int, contenido string, direccionLogica int) error {

	contenidoPagina, _ := BuscarPaginaEnCache(pid, pagina)

	if len(contenido) > buscarEspacioLibrePagina(pid, contenidoPagina) {
		clientUtils.Logger.Error("No hay espacio suficiente en la página para modificar el contenido")
		return fmt.Errorf("no hay espacio suficiente en la página para modificar el contenido")
	}

	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	for i, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			for j := 0; j < len(contenido); j++ {
				desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica + j)
				globalsCpu.Cache[i].Contenido[desplazamiento] = []byte(contenido)[j]
			}
			globalsCpu.Cache[i].Modificado = true
			//CACHE DELAY
			time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))
			clientUtils.Logger.Info(fmt.Sprintf("Cache Modify - PID %d Página %d", pid, pagina))
			return nil
		}
	}

	clientUtils.Logger.Error(fmt.Sprintf("No se encontró la entrada en caché para PID %d Página %d", pid, pagina))
	return fmt.Errorf("no se encontró la entrada en caché")
}

func buscarEspacioLibrePagina(pid int, contenido []byte) int {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	var espacioLibre int = 0

	for i := 0; i < len(contenido); i++ {
		if contenido[i] == 0 {
			espacioLibre++
		}
	}

	return espacioLibre
}

func AgregarACache(pid int, direccionLogica int, dato []byte) {
	if dato == nil {
		return
	}

	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	pagina := mmuUtils.ObtenerNumeroDePagina(direccionLogica)
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)
	tamPagina := globalsCpu.Memoria.TamanioPagina
	bytesACopiar := min(len(dato), tamPagina-desplazamiento)
	restante := dato[bytesACopiar:]
	sePasa := (desplazamiento + len(dato)) > tamPagina

	// Delay de caché
	time.Sleep(time.Millisecond * time.Duration(globalsCpu.CpuConfig.CacheDelay))

	// Copiar contenido en la página nueva
	cont := make([]byte, tamPagina)
	copy(cont[desplazamiento:], dato[:bytesACopiar])

	entrada := globalsCpu.EntradaCache{
		Pid:        pid,
		Pagina:     pagina,
		Contenido:  cont,
		Uso:        true,
		Modificado: true,
	}

	if len(globalsCpu.Cache) < globalsCpu.CpuConfig.CacheEntries {
		globalsCpu.Cache = append(globalsCpu.Cache, entrada)
		clientUtils.Logger.Info(fmt.Sprintf("Cache Add - PID %d Página %d", pid, pagina))

		if sePasa {
			nuevaDireccion := direccionLogica + bytesACopiar
			AgregarACache(pid, nuevaDireccion, restante)
		}
		return
	}

	for {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]

		switch strings.ToLower(globalsCpu.CpuConfig.CacheReplacment) {
		case "clock":
			if !actual.Uso {
				reemplazarEntradaCache(globalsCpu.PunteroClock, entrada)
				if sePasa {
					nuevaDireccion := direccionLogica + bytesACopiar
					AgregarACache(pid, nuevaDireccion, restante)
				}
				return
			}
			actual.Uso = false

		case "clock-m":
			if !actual.Uso && !actual.Modificado {
				reemplazarEntradaCache(globalsCpu.PunteroClock, entrada)
				if sePasa {
					nuevaDireccion := direccionLogica + bytesACopiar
					AgregarACache(pid, nuevaDireccion, restante)
				}
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

		contenidoBase64 := base64.StdEncoding.EncodeToString(evictada.Contenido)
		valores := []string{
			strconv.Itoa(evictada.Pid),
			strconv.Itoa(evictada.Pagina),
			contenidoBase64,
		}
		paquete := clientUtils.Paquete{Valores: valores}

		clientUtils.EnviarPaquete(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"writePagina",
			paquete,
		)

		clientUtils.Logger.Info(fmt.Sprintf("Se envió contenido a Memoria: PID %d Página %d", evictada.Pid, evictada.Pagina))
	}

	time.Sleep(time.Millisecond * time.Duration(globalsCpu.CpuConfig.CacheDelay))

	globalsCpu.Cache[indice] = nueva
	clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - PID %d Página %d → Nueva entrada", nueva.Pid, nueva.Pagina))
}

func FlushPaginasModificadas(pid int) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()
	globalsCpu.TlbMutex.Lock()
	defer globalsCpu.TlbMutex.Unlock()

	for j, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Modificado {
			marco, encontroMarco := tlbUtils.ConsultarMarco(entrada.Pagina)
			if !encontroMarco {
				clientUtils.Logger.Error(fmt.Sprintf("No se encontró el marco para la página %d del PID %d", entrada.Pagina, pid))
				continue
			}

			// Enviar bit a bit el contenido
			for i := 0; i < len(entrada.Contenido); i++ {
				bit := string(entrada.Contenido[i])
				valores := []string{
					strconv.Itoa(pid),
					strconv.Itoa(marco),
					bit,
					strconv.Itoa(i), // posición del bit en la página
				}

				// CACHE DELAY por bit
				time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))

				paquete := clientUtils.Paquete{Valores: valores}
				clientUtils.EnviarPaquete(
					globalsCpu.CpuConfig.IpMemory,
					globalsCpu.CpuConfig.PortMemory,
					"writeMemoria",
					paquete,
				)
				//Log
				clientUtils.Logger.Info(fmt.Sprintf("[Flush] PID %d: página %d, marco %d, bit pos %d -> '%s' enviado", pid, entrada.Pagina, marco, i, bit))
			}
			clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Memory Update - Página: %d - Frame: %d", pid, j, marco))
		}
	}
}

func LimpiarCache() {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	globalsCpu.Cache = []globalsCpu.EntradaCache{}
}
