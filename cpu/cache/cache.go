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
			globalsCpu.Cache[i].Uso = true
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
	sePasa := desplazamiento+len(dato) > tamPagina

	// Delay de caché
	time.Sleep(time.Millisecond * time.Duration(globalsCpu.CpuConfig.CacheDelay))

	// Leer el contenido completo de la página actual desde Memoria
	marco, err := mmuUtils.ObtenerMarco(pid, direccionLogica)
	if err != nil {
		clientUtils.Logger.Error("AgregarACache - No se pudo obtener el marco antes de leer la página")
		return
	}

	paginaCompleta, err := consultaRead(pid, marco, direccionLogica, len(dato))
	if err != nil {
		clientUtils.Logger.Error("AgregarACache - No se pudo leer la página completa antes de escribir en caché")
		return
	}

	// Reemplazar parte de la página con el nuevo contenido
	copy(paginaCompleta[desplazamiento:], dato[:bytesACopiar])

	entrada := globalsCpu.EntradaCache{
		Pid:        pid,
		Pagina:     pagina,
		Contenido:  paginaCompleta,
		Uso:        true,
		Modificado: true, // ya que agregamos nuevo contenido
	}

	if len(globalsCpu.Cache) < globalsCpu.CpuConfig.CacheEntries {
		globalsCpu.Cache = append(globalsCpu.Cache, entrada)
		clientUtils.Logger.Info(fmt.Sprintf("Cache Add - PID %d Página %d", pid, pagina))
	} else {
		cantidadEntradas := len(globalsCpu.Cache)
		vueltas := 0
		for vueltas < cantidadEntradas {
			actual := &globalsCpu.Cache[globalsCpu.PunteroClock]
			switch strings.ToLower(globalsCpu.CpuConfig.CacheReplacment) {
			case "clock":
				if !actual.Uso {
					reemplazarEntradaCache(globalsCpu.PunteroClock, entrada)
					goto check_overflow
				}
				actual.Uso = false

			case "clock-m":
				if !actual.Uso && !actual.Modificado {
					reemplazarEntradaCache(globalsCpu.PunteroClock, entrada)
					goto check_overflow
				}
				actual.Uso = false
			}
			globalsCpu.PunteroClock = (globalsCpu.PunteroClock + 1) % cantidadEntradas
			vueltas++
		}
	}

check_overflow:
	if sePasa {
		nuevaDireccion := direccionLogica + bytesACopiar
		AgregarACache(pid, nuevaDireccion, restante)
	}
}

func reemplazarEntradaCache(indice int, nueva globalsCpu.EntradaCache) {
	evictada := globalsCpu.Cache[indice]

	clientUtils.Logger.Debug("El valor de evictada es: ", "Evictada", evictada.Contenido)

	clientUtils.Logger.Debug("La cache a modificar y la que la modifica son", "Modificada", evictada, "Modificadora", nueva)

	if evictada.Modificado {
		clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - Página %d modificada → escribir en Memoria", evictada.Pagina))

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

		clientUtils.Logger.Info(fmt.Sprintf("Se envió contenido a Memoria: PID %d Página %d Marco %d", evictada.Pid, evictada.Pagina, marco))
	}

	time.Sleep(time.Millisecond * time.Duration(globalsCpu.CpuConfig.CacheDelay))

	globalsCpu.Cache[indice] = nueva
	clientUtils.Logger.Info(fmt.Sprintf("Cache Replace - PID %d Página %d → Nueva entrada", nueva.Pid, nueva.Pagina))
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

			clientUtils.Logger.Info(fmt.Sprintf("[Flush] PID %d: Página %d → Marco %d guardada en Memoria", pid, entrada.Pagina, marco))
		}
	}
}

func LimpiarCache() {
	globalsCpu.CacheMutex.Lock()
	defer globalsCpu.CacheMutex.Unlock()

	globalsCpu.Cache = []globalsCpu.EntradaCache{}
}

func consultaRead(pid int, marco int, direccionLogica int, tamanio int) ([]byte, error) {
	pageSize := globalsCpu.Memoria.TamanioPagina
	desplazamiento := mmuUtils.ObtenerDesplazamiento(direccionLogica)
	direccionFisica := marco*pageSize + desplazamiento
	if tamanio < 1 {
		valores := []string{
			strconv.Itoa(pid),
			strconv.Itoa(direccionFisica),
		}
		paquete := clientUtils.Paquete{Valores: valores}

		respuesta := clientUtils.EnviarPaqueteConRespuestaBody(
			globalsCpu.CpuConfig.IpMemory,
			globalsCpu.CpuConfig.PortMemory,
			"readMemoria",
			paquete,
		)

		if respuesta == nil || len(respuesta) == 0 {
			return nil, fmt.Errorf("no se pudo obtener dato en readMemoria")
		}

		return respuesta, nil
	}

	// Lectura de página completa
	valores := []string{
		strconv.Itoa(pid),
		strconv.Itoa(marco),
		strconv.Itoa(pageSize),
	}
	paquete := clientUtils.Paquete{Valores: valores}

	paginaCompleta := clientUtils.EnviarPaqueteConRespuestaBody(
		globalsCpu.CpuConfig.IpMemory,
		globalsCpu.CpuConfig.PortMemory,
		"readPagina",
		paquete,
	)

	if len(paginaCompleta) != pageSize {
		return nil, fmt.Errorf("tamaño de página recibido incorrecto")
	}

	if desplazamiento+tamanio > pageSize {
		return nil, fmt.Errorf("rango de lectura excede el tamaño de página")
	}

	return paginaCompleta, nil
}
