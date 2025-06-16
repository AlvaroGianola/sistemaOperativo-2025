package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	globalsMemoria "github.com/sisoputnfrba/tp-golang/memoria/globalsMemoria"
	memoriaUtils "github.com/sisoputnfrba/tp-golang/memoria/memoriaUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

type Paquete struct {
	Valores []string `json:"valores"`
}

type InfoMemoria struct {
	TamPagina        int `json:"tam_pagina"`
	Niveles          int `json:"niveles"`
	EntradasPorTabla int `json:"entradas_por_tabla"`
}

func main() {

	// Inicializa el logger que usará todo el módulo Memoria
	clientUtils.ConfigurarLogger("memoria.log")

	// Carga la configuración desde el archivo config.json
	globalsMemoria.MemoriaConfig = memoriaUtils.IniciarConfiguracion("config.json")

	// Crea el multiplexer HTTP y registra los endpoints que usará Memoria
	mux := http.NewServeMux()

	// Levanta el servidor en el puerto definido por configuración
	direccion := fmt.Sprintf("%s:%d", globalsMemoria.MemoriaConfig.IpMemory, globalsMemoria.MemoriaConfig.PortMemory)
	fmt.Printf("[Memoria] Servidor escuchando en puerto %d...\n", globalsMemoria.MemoriaConfig.PortMemory)

	mux.HandleFunc("/obtenerEntradaTabla", entradaTabla)
	mux.HandleFunc("/accederMarcoUsuario", accederMarcoUsuario)
	mux.HandleFunc("/readMemoria", readMemoria)
	mux.HandleFunc("/writeMemoria", writeMemoria)
	mux.HandleFunc("/readPagina", readPagina)
	mux.HandleFunc("/obtenerTamPagina", tamPagina)
	mux.HandleFunc("/siguienteInstruccion", memoriaUtils.SiguienteInstruccion)
	mux.HandleFunc("/iniciarProceso", memoriaUtils.IniciarProceso)
	mux.HandleFunc("/finalizarProceso", memoriaUtils.FinalizarProceso)

	err := http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}

	clientUtils.Logger.Info("🔧 Mock Memoria corriendo en : %s", direccion)

}

func entradaTabla(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	nivel, _ := strconv.Atoi(p.Valores[2])
	entrada, _ := strconv.Atoi(p.Valores[3])

	id := (nivel * 100) + entrada
	w.Write([]byte(fmt.Sprintf("%d", id)))
}

func accederMarcoUsuario(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	fmt.Printf("📦 accederMarcoUsuario - Recibido: %+v\n", p.Valores)
	pagina, _ := strconv.Atoi(p.Valores[0])
	w.Write([]byte(fmt.Sprintf("%d", 1000+pagina))) // Marco ficticio
}

func readMemoria(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	tam, _ := strconv.Atoi(p.Valores[2])
	contenido := "X"
	result := ""
	for i := 0; i < tam; i++ {
		result += contenido
	}
	w.Write([]byte(result))
}

func writeMemoria(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	log.Printf("WRITE: PID %s Marco %s Dato %s\n", p.Valores[0], p.Valores[1], p.Valores[2])
	w.Write([]byte("OK"))
}

func readPagina(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	pagina := p.Valores[1]
	w.Write([]byte("ContenidoDePagina" + pagina))
}

func tamPagina(w http.ResponseWriter, r *http.Request) {
	// Devuelve [tamaño_pagina, niveles, entradas]
	tamanioDePagina := 64
	niveles := 2
	entradas := 256
	tam := []byte{byte(tamanioDePagina), byte(niveles), byte(entradas)}
	w.Write([]byte(tam)) // Simula un JSON array
}
