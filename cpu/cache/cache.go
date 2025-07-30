package cache

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	globalsCpu "github.com/sisoputnfrba/tp-golang/cpu/globalsCpu"
	mmuUtils "github.com/sisoputnfrba/tp-golang/cpu/mmu"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func BuscarPaginaEnCache(pid int, pagina int) ([]byte, bool) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	// Buscar en la caché
	for i, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			//CACHE DELAY
			globalsCpu.Cache[i].Uso = true
			time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))
			clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache HIT - Pagina: %d", pid, pagina))
			return entrada.Contenido, true
		}
	}

	clientUtils.Logger.Info(fmt.Sprintf("PID: %d - Cache MISS - Pagina: %d", pid, pagina))
	return nil, false
}

func ModificarContenidoCache(pid int, pagina int, contenido string, direccionLogica int) error {

	//contenidoPagina, _ := BuscarPaginaEnCache(pid, pagina)

	/*---
	if len(contenido) > buscarEspacioLibrePagina(pid, contenidoPagina) {
		clientUtils.Logger.Error("No hay espacio suficiente en la página para modificar el contenido")
		return fmt.Errorf("no hay espacio suficiente en la página para modificar el contenido")
	}*/

	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	for i, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Pagina == pagina {
			for j := 0; j < len(contenido); j++ {
				desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica + j)
				globalsCpu.Cache[i].Contenido[desplazamiento] = []byte(contenido)[j]
			}
			globalsCpu.Cache[i].Uso = true
			globalsCpu.Cache[i].Modificado = true
			//CACHE DELAY
			time.Sleep(time.Duration(globalsCpu.CpuConfig.CacheDelay))
			//clientUtils.Logger.Info(fmt.Sprintf("Cache Modify - PID %d Página %d", pid, pagina))
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
		if contenido[i] == 0x00 {
			espacioLibre++
		}
	}

	return espacioLibre
}

func AgregarACache(pid int, direccionLogica int, dato []byte, modifica bool) {
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
	sePasa := desplazamiento+len(dato) > tamPagina

	// Delay de caché
	time.Sleep(time.Millisecond * time.Duration(globalsCpu.CpuConfig.CacheDelay))

	// Leer el contenido completo de la página actual desde Memoria
	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error("AgregarACache - No se pudo obtener el marco antes de leer la página")
		return
	}

	paginaCompleta, err := consultaRead(pid, marco)

	if err != nil || len(paginaCompleta) != tamPagina {
		clientUtils.Logger.Error("AgregarACache - No se pudo leer la página completa antes de escribir en caché")
		return
	}

	// Reemplazar parte de la página con el nuevo contenido
	copy(paginaCompleta[desplazamiento:], dato[:bytesACopiar])

	nuevaEntrada := globalsCpu.EntradaCache{
		Pid:        pid,
		Pagina:     pagina,
		Contenido:  paginaCompleta,
		Uso:        true,
		Modificado: modifica,
	}

	if len(globalsCpu.Cache) < globalsCpu.CpuConfig.CacheEntries {
		globalsCpu.Cache = append(globalsCpu.Cache, nuevaEntrada)
		clientUtils.Logger.Info(fmt.Sprintf("PID %d - Cache Add - Página %d", pid, pagina))
	} else {

		for i := range globalsCpu.CpuConfig.CacheEntries {
			clientUtils.Logger.Debug("Paginas en cache con uso y modificado", "Pagina", globalsCpu.Cache[i].Pagina, "Uso", globalsCpu.Cache[i].Uso, "Modificado", globalsCpu.Cache[i].Modificado)
		}
		//clientUtils.Logger.Debug("Puntero en : ", "Puntero", globalsCpu.PunteroClock)
		switch strings.ToLower(globalsCpu.CpuConfig.CacheReplacment) {
		case "clock":
			reemplazarPorClock(nuevaEntrada)
		case "clock-m":
			reemplazarPorClockM(nuevaEntrada, pid, direccionLogica)
		default:
			clientUtils.Logger.Error("Algoritmo de reemplazo de cache no reconocido")
		}
	}

	// Si el contenido se desbordó a otra página, escribimos el resto recursivamente
	if sePasa {
		nuevaDireccion := direccionLogica + bytesACopiar
		AgregarACache(pid, nuevaDireccion, restante, modifica)
	}
}

func avanzarPuntero() {
	globalsCpu.PunteroClock = (globalsCpu.PunteroClock + 1) % len(globalsCpu.Cache)
}

func reemplazarPorClock(nuevaEntrada globalsCpu.EntradaCache) {
	cantidadEntradas := len(globalsCpu.Cache)

	// Primera vuelta: buscar Uso = false
	for i := 0; i < cantidadEntradas; i++ {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if !actual.Uso {
			reemplazarEntradaCache(globalsCpu.PunteroClock, nuevaEntrada)
			avanzarPuntero()
			return
		}
		actual.Uso = false
		avanzarPuntero()
	}

	// Segunda vuelta: ahora alguna tendrá Uso = false
	for i := 0; i < cantidadEntradas; i++ {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if !actual.Uso {
			reemplazarEntradaCache(globalsCpu.PunteroClock, nuevaEntrada)
			avanzarPuntero()
			return
		}
		avanzarPuntero()
	}
}

func reemplazarPorClockM(nuevaEntrada globalsCpu.EntradaCache, pid int, direccionLogica int) {
	cantidad := len(globalsCpu.Cache)
	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)

	if err != nil {
		clientUtils.Logger.Error("error al obtener el marco")
	}

	// Fase 1: Buscar Uso=false y Modificado=false
	for i := 0; i < cantidad; i++ {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if !actual.Uso && !actual.Modificado {
			clientUtils.Logger.Debug(fmt.Sprintf("CLOCK-M FASE 1 - Reemplazo limpio de página %d", actual.Pagina))
			reemplazarEntradaCache(globalsCpu.PunteroClock, nuevaEntrada)
			avanzarPuntero()
			return
		}
		avanzarPuntero()
	}

	// Fase 2: Buscar Uso=false y Modificado=true → escribo antes (Si su Uso=true lo setteo en false)
	for i := 0; i < cantidad; i++ {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if !actual.Uso && actual.Modificado {
			clientUtils.Logger.Debug(fmt.Sprintf("CLOCK-M FASE 2 - Reemplazo de página modificada %d", actual.Pagina))
			consultaWrite(actual.Pid, marco, actual.Pagina*globalsCpu.Memoria.TamanioPagina, actual.Contenido)
			reemplazarEntradaCache(globalsCpu.PunteroClock, nuevaEntrada)
			avanzarPuntero()
			return
		} else if !actual.Uso {
			actual.Uso = true
		}
		avanzarPuntero()
	}

	// Si no encuentro ninguno con Uso=false y Modificado=true Vuelvo a hacer fase 1
	// Fase 3: Buscar Uso=false y Modificado=false
	for i := 0; i < cantidad; i++ {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if !actual.Uso && !actual.Modificado {
			clientUtils.Logger.Debug(fmt.Sprintf("CLOCK-M FASE 1 - Reemplazo limpio de página %d", actual.Pagina))
			reemplazarEntradaCache(globalsCpu.PunteroClock, nuevaEntrada)
			avanzarPuntero()
			return
		}
		avanzarPuntero()
	}

	// Vuelvo a hacer fase 2
	// Fase 4: Buscar Uso=false y Modificado=true → escribir antes y reemplazar(Si o si tiene que haber 1)
	for i := 0; i < cantidad; i++ {
		actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
		if !actual.Uso && actual.Modificado {
			clientUtils.Logger.Debug(fmt.Sprintf("CLOCK-M FASE 2 - Reemplazo de página modificada %d", actual.Pagina))
			consultaWrite(actual.Pid, marco, actual.Pagina*globalsCpu.Memoria.TamanioPagina, actual.Contenido)
			reemplazarEntradaCache(globalsCpu.PunteroClock, nuevaEntrada)
			avanzarPuntero()
			return
		} else if !actual.Uso {
			actual.Uso = true
		}
		avanzarPuntero()
	}
}

func reemplazarEntradaCache(indice int, nueva globalsCpu.EntradaCache) {
	evictada := globalsCpu.Cache[indice]

	//clientUtils.Logger.Debug("El valor de evictada es: ", "Evictada", evictada.Contenido)

	//clientUtils.Logger.Debug("La cache a modificar y la que la modifica son", "Modificada", evictada, "Modificadora", nueva)

	if evictada.Modificado {
		//clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - Página %d modificada → escribir en Memoria", evictada.Pagina))

		// 🔍 Buscar marco real
		direccionLogica := mmuUtils.ObtenerDireccionLogica(evictada.Pagina)
		marco, err := mmuUtils.ObtenerMarco(evictada.Pid, direccionLogica)
		if err != nil {
			clientUtils.Logger.Error(fmt.Sprintf("No se pudo obtener marco de la página %d del PID %d", evictada.Pagina, evictada.Pid))
			return
		}

		// Leer toda la página original desde Memoria
		valoresLeer := []string{
			strconv.Itoa(evictada.Pid),
			strconv.Itoa(marco),
			strconv.Itoa(len(evictada.Contenido)),
		}
		paqueteLeer := clientUtils.Paquete{Valores: valoresLeer}

		paginaCompleta := clientUtils.EnviarPaqueteConRespuestaBody(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"readPagina",
			paqueteLeer,
		)

		for i := 0; i < len(evictada.Contenido) && i < len(paginaCompleta); i++ {
			if evictada.Contenido[i] == 0x00 {
				evictada.Contenido[i] = paginaCompleta[i]
			}
		}

		// Armás el paquete para mandarlo a Memoria
		valores := []string{
			strconv.Itoa(evictada.Pid),
			strconv.Itoa(marco),
			strconv.Itoa(len(evictada.Contenido)),
		}
		for _, b := range evictada.Contenido {
			valores = append(valores, strconv.Itoa(int(b)))
		}

		paquete := clientUtils.Paquete{Valores: valores}
		clientUtils.EnviarPaquete(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"writePagina",
			paquete,
		)

		clientUtils.Logger.Info(fmt.Sprintf("PID %d - Memory Update - Página %d - Frame %d", evictada.Pid, evictada.Pagina, marco))
	}

	time.Sleep(time.Millisecond * time.Duration(globalsCpu.CpuConfig.CacheDelay))

	globalsCpu.Cache[indice] = nueva
	clientUtils.Logger.Info(fmt.Sprintf("PID %d - Cache Add - Página %d", nueva.Pid, nueva.Pagina))
}

func FlushPaginasModificadas(pid int) {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	for _, entrada := range globalsCpu.Cache {
		if entrada.Pid == pid && entrada.Modificado {
			marco, err := mmuUtils.ObtenerMarco(entrada.Pid, mmuUtils.ObtenerDireccionLogica(entrada.Pagina))
			if err != nil {
				clientUtils.Logger.Error(fmt.Sprintf("No se encontró el marco para la página %d del PID %d", entrada.Pagina, pid))
				continue
			}

			valores := []string{
				strconv.Itoa(pid),
				strconv.Itoa(marco),
				strconv.Itoa(len(entrada.Contenido)),
			}
			for _, b := range entrada.Contenido {
				valores = append(valores, strconv.Itoa(int(b)))
			}

			paquete := clientUtils.Paquete{Valores: valores}
			clientUtils.EnviarPaquete(
				globalsCpu.CpuConfig.IpMemory,
				globalsCpu.CpuConfig.PortMemory,
				"writePagina",
				paquete,
			)

			clientUtils.Logger.Info(fmt.Sprintf("PID %d - Memory Update - Página %d Marco %d", pid, entrada.Pagina, marco))
		}
	}
}

func LimpiarCache() {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	globalsCpu.Cache = []globalsCpu.EntradaCache{}
}

func consultaRead(pid int, marco int) ([]byte, error) {
	pageSize := globalsCpu.Memoria.TamanioPagina
	// Lectura de página completa
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	paquete := clientUtils.Paquete{Valores: valores}

	//clientUtils.Logger.Debug("Paquetes a enviar", "Paquete", paquete)

	paginaCompleta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paquete,
	)

	if len(paginaCompleta) != pageSize {
		return nil, fmt.Errorf("tamaño de página recibido incorrecto")
	}

	return paginaCompleta, nil
}

func consultaWrite(pid int, marco int, direccionLogica int, datos []byte) error {
	pageSize := globalsCpu.Memoria.TamanioPagina
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)

	// Primero leemos toda la página para modificar solo los bytes necesarios
	valoresLeer := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	paqueteLeer := clientUtils.Paquete{Valores: valoresLeer}

	paginaCompleta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paqueteLeer,
	)

	if len(paginaCompleta) != pageSize {
		return fmt.Errorf("no se pudo leer página completa antes de escribir")
	}

	// Modificar solo el rango correspondiente
	copy(paginaCompleta[desplazamiento:], datos)

	// Preparar paquete para escribir página completa
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	for _, b := range paginaCompleta {
		valores = append(valores, strconv.Itoa(int(b)))
	}
	paquete := clientUtils.Paquete{Valores: valores}

	clientUtils.EnviarPaquete(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"writePagina",
		paquete,
	)

	return nil
}
